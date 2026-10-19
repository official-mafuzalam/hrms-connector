package relay

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hrms-connector/internal/config"
	"hrms-connector/internal/key"
)

type ConfigProvider interface {
	Snapshot() config.Config
}

type LogWriter interface {
	Printf(format string, v ...any)
}

type Device struct {
	SN       string    `json:"sn"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
	Count    uint64    `json:"count"`
}

type Counters struct {
	Requests    uint64 `json:"requests"`
	Errors      uint64 `json:"errors"`
	Upstream503 uint64 `json:"upstream_503"`
}

type Stats struct {
	Counters
	Devices []Device `json:"devices"`
}

type Proxy struct {
	config    ConfigProvider
	logger    LogWriter
	mu        sync.Mutex
	stats     Counters
	devices   map[string]Device
	transMu   sync.Mutex
	transKey  string
	transport *http.Transport
}

func New(cfg ConfigProvider, logger LogWriter) *Proxy {
	return &Proxy{config: cfg, logger: logger, devices: make(map[string]Device)}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/iclock/") && r.URL.Path != "/iclock" {
		http.NotFound(w, r)
		return
	}
	cfg := p.config.Snapshot()
	start := time.Now()
	sn := r.URL.Query().Get("SN")
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	status := 0
	defer func() {
		if p.logger != nil {
			p.logger.Printf("%s SN=%q %s %s status=%d ms=%d", start.Format(time.RFC3339), sn, r.Method, r.URL.Path, status, time.Since(start).Milliseconds())
		}
	}()

	p.mu.Lock()
	p.stats.Requests++
	p.mu.Unlock()

	if !allowed(ip, cfg.AllowedCIDRs) {
		p.fail()
		status = http.StatusForbidden
		http.Error(w, "forbidden", status)
		return
	}
	p.mu.Lock()
	p.recordDevice(sn, ip, start)
	p.mu.Unlock()
	maxBytes := int64(cfg.MaxBodyKB) * 1024
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			p.fail()
			status = http.StatusRequestEntityTooLarge
			http.Error(w, "request body too large", status)
			return
		}
		p.fail()
		status = http.StatusBadRequest
		http.Error(w, "invalid request body", status)
		return
	}
	conn, err := key.Parse(cfg.Key)
	if err != nil {
		p.fail()
		status = http.StatusServiceUnavailable
		http.Error(w, "upstream unavailable", status)
		return
	}
	target, err := upstreamURL(conn.URL, r.URL)
	if err != nil {
		p.fail()
		status = http.StatusServiceUnavailable
		http.Error(w, "upstream unavailable", status)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		p.fail()
		status = http.StatusServiceUnavailable
		http.Error(w, "upstream unavailable", status)
		return
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header["Accept-Encoding"] = r.Header.Values("Accept-Encoding")
	req.Header.Set("X-Connector-Token", conn.Token)
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := p.client(cfg).Do(req)
	if err != nil {
		p.fail()
		status = http.StatusServiceUnavailable
		p.mu.Lock()
		p.stats.Upstream503++
		p.mu.Unlock()
		http.Error(w, "upstream unavailable", status)
		return
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	if status >= http.StatusBadRequest {
		p.fail()
	}
	if status == http.StatusServiceUnavailable {
		p.mu.Lock()
		p.stats.Upstream503++
		p.mu.Unlock()
	}
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(status)
	if _, err := io.Copy(w, resp.Body); err != nil {
		p.fail()
		return
	}
}

func (p *Proxy) recordDevice(sn, ip string, seen time.Time) {
	if sn == "" {
		return
	}
	device, ok := p.devices[sn]
	if !ok && len(p.devices) >= 500 {
		oldestSN := ""
		var oldest time.Time
		for serial, entry := range p.devices {
			if oldestSN == "" || entry.LastSeen.Before(oldest) {
				oldestSN, oldest = serial, entry.LastSeen
			}
		}
		delete(p.devices, oldestSN)
	}
	device.SN, device.IP, device.LastSeen = sn, ip, seen
	device.Count++
	p.devices[sn] = device
}

func (p *Proxy) fail() {
	p.mu.Lock()
	p.stats.Errors++
	p.mu.Unlock()
}

func (p *Proxy) Snapshot() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := Stats{Counters: p.stats, Devices: make([]Device, 0, len(p.devices))}
	for _, device := range p.devices {
		out.Devices = append(out.Devices, device)
	}
	sort.Slice(out.Devices, func(i, j int) bool { return out.Devices[i].SN < out.Devices[j].SN })
	return out
}

func (p *Proxy) client(cfg config.Config) *http.Client {
	transportKey := cfg.HTTPProxy + "|" + strconv.FormatBool(cfg.TLSVerify)
	p.transMu.Lock()
	defer p.transMu.Unlock()
	if p.transport == nil || p.transKey != transportKey {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !cfg.TLSVerify} //nolint:gosec
		transport.DisableCompression = true
		if cfg.HTTPProxy != "" {
			proxyURL, _ := url.Parse(cfg.HTTPProxy)
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			transport.Proxy = http.ProxyFromEnvironment
		}
		if p.transport != nil {
			p.transport.CloseIdleConnections()
		}
		p.transport, p.transKey = transport, transportKey
	}
	return &http.Client{Transport: p.transport}
}

func allowed(ip string, cidrs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

func upstreamURL(base, incoming *url.URL) (*url.URL, error) {
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + incoming.Path
	target.RawPath = strings.TrimRight(base.EscapedPath(), "/") + incoming.EscapedPath()
	if target.RawPath == target.Path {
		target.RawPath = ""
	}
	target.RawQuery = incoming.RawQuery
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid upstream URL")
	}
	return &target, nil
}

func copyHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}
