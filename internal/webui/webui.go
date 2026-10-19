package webui

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"hrms-connector/internal/config"
	"hrms-connector/internal/heartbeat"
	"hrms-connector/internal/key"
	"hrms-connector/internal/logging"
	"hrms-connector/internal/netcfg"
	"hrms-connector/internal/relay"
	"hrms-connector/internal/version"
)

//go:embed static/*
var assets embed.FS

type ConfigStore interface {
	Snapshot() config.Config
	Update(config.Config) error
}

type HeartbeatState interface {
	Snapshot() heartbeat.Status
}

type StatsProvider interface {
	Snapshot() relay.Stats
}

type NetworkManager interface {
	Status(context.Context) (netcfg.Network, error)
	StatusForDevices(context.Context, []string) (netcfg.Network, error)
	SelectAdapter(context.Context, string) error
	Apply(context.Context, bool) (netcfg.Network, error)
}

type ListenerHealth interface {
	Snapshot() (bool, string)
}

type UI struct {
	store     ConfigStore
	heartbeat HeartbeatState
	stats     StatsProvider
	logger    *logging.Logger
	network   NetworkManager
	listeners ListenerHealth
	dataDir   string
	mux       *http.ServeMux
	mu        sync.Mutex
	setupMu   sync.Mutex
	setupKey  string
	setupIPs  []string
	setupNote string
	sessions  map[string]csrfSession
	failures  map[string]failureState
}

type csrfSession struct {
	token   string
	expires time.Time
}

type failureState struct {
	count        int
	start        time.Time
	blockedUntil time.Time
}

func New(store ConfigStore, hb HeartbeatState, stats StatsProvider, logger *logging.Logger, dataDir string, networkManagers ...NetworkManager) *UI {
	var network NetworkManager
	if len(networkManagers) > 0 {
		network = networkManagers[0]
	}
	u := &UI{
		store: store, heartbeat: hb, stats: stats, logger: logger, dataDir: dataDir,
		network: network,
		mux:     http.NewServeMux(), sessions: make(map[string]csrfSession), failures: make(map[string]failureState),
	}
	u.mux.HandleFunc("/", u.serveHome)
	u.mux.HandleFunc("/style.css", u.serveAsset)
	u.mux.HandleFunc("/app.js", u.serveAsset)
	u.mux.HandleFunc("/api/session", u.session)
	u.mux.HandleFunc("/api/status", u.status)
	u.mux.HandleFunc("/api/setup", u.setupData)
	u.mux.HandleFunc("/api/setup-pin", u.protect(u.setupPIN))
	u.mux.HandleFunc("/api/verify-key", u.protect(u.verifyKey))
	u.mux.HandleFunc("/api/check-port", u.protect(u.checkPort))
	u.mux.HandleFunc("/api/settings", u.protect(u.saveSettings))
	u.mux.HandleFunc("/api/netfix", u.protect(u.fixNetwork))
	u.mux.HandleFunc("/api/select-adapter", u.protect(u.selectAdapter))
	u.mux.HandleFunc("/api/logs", u.logs)
	u.mux.HandleFunc("/api/logs/download", u.downloadLogs)
	u.mux.HandleFunc("/api/about", u.about)
	return u
}

func (u *UI) SetListenerHealth(health ListenerHealth) {
	u.listeners = health
}

func (u *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowedHost(r.Host) {
		http.Error(w, "invalid host", http.StatusForbidden)
		return
	}
	u.mux.ServeHTTP(w, r)
}

func allowedHost(hostport string) bool {
	host := hostport
	if parsedHost, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsedHost
	}
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1"
}

func (u *UI) serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/setup" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	if r.URL.Path == "/" && u.store.Snapshot().Key == "" {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	content, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "interface unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprint(len(content)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(content)
}

func (u *UI) serveAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	content, err := assets.ReadFile("static" + r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(r.URL.Path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(content)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(content)
}

func (u *UI) session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sessionID, err := randomToken(32)
	if err != nil {
		http.Error(w, "could not initialize session", http.StatusInternalServerError)
		return
	}
	csrf, err := randomToken(32)
	if err != nil {
		http.Error(w, "could not initialize session", http.StatusInternalServerError)
		return
	}
	u.mu.Lock()
	for id, session := range u.sessions {
		if time.Now().After(session.expires) {
			delete(u.sessions, id)
		}
	}
	if len(u.sessions) >= 1000 {
		u.mu.Unlock()
		http.Error(w, "too many local UI sessions; try again later", http.StatusServiceUnavailable)
		return
	}
	u.sessions[sessionID] = csrfSession{token: csrf, expires: time.Now().Add(8 * time.Hour)}
	u.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "hrms_session", Value: sessionID, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: 8 * 60 * 60,
	})
	writeJSON(w, map[string]string{"csrf": csrf})
}

func (u *UI) protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		origin, err := url.Parse(r.Header.Get("Origin"))
		if err != nil || origin.Host != r.Host || origin.Scheme != "http" {
			http.Error(w, "same-origin request required", http.StatusForbidden)
			return
		}
		cookie, err := r.Cookie("hrms_session")
		if err != nil {
			http.Error(w, "session required", http.StatusForbidden)
			return
		}
		u.mu.Lock()
		session, ok := u.sessions[cookie.Value]
		u.mu.Unlock()
		if !ok || time.Now().After(session.expires) || session.token == "" || r.Header.Get("X-CSRF-Token") != session.token {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		cfg := u.store.Snapshot()
		if cfg.PINHash != "" {
			if !u.checkPIN(w, r, cfg.PINHash) {
				return
			}
		}
		next(w, r)
	}
}

func (u *UI) checkPIN(w http.ResponseWriter, r *http.Request, hash string) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	now := time.Now()
	u.mu.Lock()
	for failedIP, entry := range u.failures {
		if now.Sub(entry.start) > 10*time.Minute && now.After(entry.blockedUntil) {
			delete(u.failures, failedIP)
		}
	}
	failure := u.failures[ip]
	if now.Before(failure.blockedUntil) {
		u.mu.Unlock()
		http.Error(w, "too many PIN attempts; try again later", http.StatusTooManyRequests)
		return false
	}
	if now.Sub(failure.start) > 5*time.Minute {
		failure = failureState{start: now}
	}
	u.mu.Unlock()
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.Header.Get("X-UI-PIN")))
	if err == nil {
		u.mu.Lock()
		delete(u.failures, ip)
		u.mu.Unlock()
		return true
	}
	u.mu.Lock()
	failure = u.failures[ip]
	if now.Sub(failure.start) > 5*time.Minute {
		failure = failureState{start: now}
	}
	failure.count++
	if failure.count >= 5 {
		failure.blockedUntil = now.Add(5 * time.Minute)
	}
	u.failures[ip] = failure
	u.mu.Unlock()
	http.Error(w, "incorrect PIN", http.StatusUnauthorized)
	return false
}

func (u *UI) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cfg := u.store.Snapshot()
	w.Header().Set("Cache-Control", "no-store")
	stats := u.stats.Snapshot()
	listening, listenError := true, ""
	if u.listeners != nil {
		listening, listenError = u.listeners.Snapshot()
	}
	var network any
	var networkError string
	if u.network != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		addressing, err := u.network.StatusForDevices(ctx, deviceIPs(stats.Devices))
		cancel()
		network = addressing
		if err != nil {
			networkError = err.Error()
		}
	}
	writeJSON(w, map[string]any{
		"heartbeat": u.heartbeat.Snapshot(), "stats": stats, "version": version.Version,
		"language": cfg.Language, "port": cfg.Port, "ui_port": cfg.UIPort,
		"allowed_cidrs": cfg.AllowedCIDRs, "upstream_timeout": cfg.UpstreamTimeout,
		"max_body_kb": cfg.MaxBodyKB, "tls_verify": cfg.TLSVerify,
		"http_proxy": cfg.HTTPProxy, "log_level": cfg.LogLevel,
		"key_masked": key.Masked(cfg.Key), "lan_addresses": LANAddresses(),
		"devices":         stats.Devices,
		"relay_listening": listening, "relay_listener_error": listenError,
		"netcfg": network, "netcfg_error": networkError,
	})
}

func (u *UI) fixNetwork(w http.ResponseWriter, r *http.Request) {
	if u.network == nil {
		http.Error(w, "network configuration is unavailable on this platform", http.StatusNotImplemented)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	network, err := u.network.Apply(ctx, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, network)
}

func (u *UI) selectAdapter(w http.ResponseWriter, r *http.Request) {
	if u.network == nil {
		http.Error(w, "adapter selection is unavailable on this platform", http.StatusNotImplemented)
		return
	}
	var input struct {
		Interface string `json:"interface"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil || strings.TrimSpace(input.Interface) == "" {
		http.Error(w, "invalid adapter selection", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := u.network.SelectAdapter(ctx, input.Interface); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func deviceIPs(devices []relay.Device) []string {
	ips := make([]string, 0, len(devices))
	for _, device := range devices {
		ips = append(ips, device.IP)
	}
	return ips
}

func (u *UI) setupPIN(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		http.Error(w, "invalid PIN request", http.StatusBadRequest)
		return
	}
	if len([]byte(input.PIN)) > 72 {
		http.Error(w, "PIN must be no longer than 72 bytes", http.StatusBadRequest)
		return
	}
	cfg := u.store.Snapshot()
	if input.PIN == "" {
		cfg.PINHash = ""
	} else {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.PIN), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "could not save PIN", http.StatusInternalServerError)
			return
		}
		cfg.PINHash = string(hash)
	}
	if err := u.store.Update(cfg); err != nil {
		http.Error(w, "could not save PIN", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (u *UI) verifyKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	cfg := u.store.Snapshot()
	status, err := heartbeat.VerifyKey(r.Context(), cfg, strings.TrimSpace(input.Key))
	if err != nil {
		writeVerificationError(w, err, cfg.Language)
		return
	}
	writeJSON(w, map[string]string{"company": status.Company, "site": status.Site})
}

func (u *UI) checkPort(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil || input.Port < 1 || input.Port > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	if input.Port == u.store.Snapshot().Port {
		writeJSON(w, map[string]bool{"available": true})
		return
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", input.Port))
	if err != nil {
		writeJSON(w, map[string]bool{"available": false})
		return
	}
	if err := listener.Close(); err != nil {
		http.Error(w, "could not release port check", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"available": true})
}

func (u *UI) saveSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Key             string   `json:"key"`
		Port            int      `json:"port"`
		Language        string   `json:"language"`
		PIN             string   `json:"pin"`
		ClearPIN        bool     `json:"clear_pin"`
		AllowedCIDRs    []string `json:"allowed_cidrs"`
		UpstreamTimeout int      `json:"upstream_timeout"`
		MaxBodyKB       int      `json:"max_body_kb"`
		TLSVerify       bool     `json:"tls_verify"`
		HTTPProxy       string   `json:"http_proxy"`
		LogLevel        string   `json:"log_level"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024)).Decode(&input); err != nil {
		http.Error(w, "invalid settings", http.StatusBadRequest)
		return
	}
	cfg := u.store.Snapshot()
	if input.Key != "" {
		if _, err := key.Parse(input.Key); err != nil {
			writeVerificationError(w, &heartbeat.VerificationError{Kind: "invalid_key", Details: err.Error()}, cfg.Language)
			return
		}
		if input.Key != cfg.Key {
			if _, err := heartbeat.VerifyKey(r.Context(), cfg, input.Key); err != nil {
				writeVerificationError(w, err, cfg.Language)
				return
			}
		}
		cfg.Key = input.Key
	}
	cfg.Port, cfg.Language = input.Port, input.Language
	cfg.AllowedCIDRs = input.AllowedCIDRs
	cfg.UpstreamTimeout, cfg.MaxBodyKB = input.UpstreamTimeout, input.MaxBodyKB
	cfg.TLSVerify, cfg.HTTPProxy, cfg.LogLevel = input.TLSVerify, input.HTTPProxy, input.LogLevel
	if input.ClearPIN {
		cfg.PINHash = ""
	} else if input.PIN != "" {
		if len([]byte(input.PIN)) > 72 {
			http.Error(w, "PIN must be no longer than 72 bytes", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.PIN), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "could not save PIN", http.StatusInternalServerError)
			return
		}
		cfg.PINHash = string(hash)
	}
	if err := cfg.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.Port != u.store.Snapshot().Port {
		listener, err := net.Listen("tcp", net.JoinHostPort(cfg.Bind, fmt.Sprint(cfg.Port)))
		if err != nil {
			http.Error(w, "device relay port is unavailable", http.StatusConflict)
			return
		}
		if err := listener.Close(); err != nil {
			http.Error(w, "could not release device relay port check", http.StatusInternalServerError)
			return
		}
	}
	if err := u.store.Update(cfg); err != nil {
		http.Error(w, "could not save settings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "key_masked": key.Masked(cfg.Key)})
}

func writeVerificationError(w http.ResponseWriter, err error, language string) {
	var verificationError *heartbeat.VerificationError
	if !errors.As(err, &verificationError) {
		verificationError = &heartbeat.VerificationError{Kind: "unexpected_response", Details: err.Error()}
	}
	data, marshalErr := json.Marshal(map[string]string{
		"message":    verificationError.Message(language),
		"message_en": verificationError.Message("en"),
		"message_bn": verificationError.Message("bn"),
		"details":    verificationError.Details,
	})
	if marshalErr != nil {
		http.Error(w, "could not encode verification error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write(append(data, '\n'))
}

func (u *UI) logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	lines, err := u.logger.Tail(300)
	if err != nil {
		http.Error(w, "could not read logs", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string][]string{"lines": lines})
}

func (u *UI) downloadLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	http.ServeFile(w, r, u.logger.Path())
}

func (u *UI) about(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, map[string]string{"version": version.Version, "data_dir": u.dataDir, "os": runtime.GOOS})
}

func LANAddresses() []string {
	primary := ""
	var addresses []string
	if conn, err := net.Dial("udp", "192.0.2.1:80"); err == nil {
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && !addr.IP.IsLoopback() {
			primary = addr.IP.String()
		}
		conn.Close()
	}
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			list, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, address := range list {
				var ip net.IP
				switch addr := address.(type) {
				case *net.IPNet:
					ip = addr.IP
				case *net.IPAddr:
					ip = addr.IP
				}
				if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
					addresses = append(addresses, ip.String())
				}
			}
		}
	}
	sort.Strings(addresses)
	result := make([]string, 0, len(addresses)+1)
	seen := make(map[string]bool)
	if primary != "" {
		result = append(result, primary)
		seen[primary] = true
	}
	for _, address := range addresses {
		if !seen[address] {
			seen[address] = true
			result = append(result, address)
		}
	}
	return result
}

func randomToken(bytesCount int) (string, error) {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeJSON(w http.ResponseWriter, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(append(data, '\n')); err != nil {
		return
	}
}
