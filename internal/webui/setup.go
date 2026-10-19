package webui

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"time"

	"hrms-connector/internal/netcfg"
	"hrms-connector/internal/relay"
)

type setupProbe func(context.Context, string) (bool, error)

func (u *UI) setupData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cfg := u.store.Snapshot()
	stats := u.stats.Snapshot()
	var network netcfg.Network
	var networkError string
	var suggested []string
	suggestionNote := "Suggestions are best effort; verify availability with your network administrator."
	if u.network != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		var err error
		network, err = u.network.StatusForDevices(ctx, deviceIPs(stats.Devices))
		cancel()
		if err != nil {
			networkError = err.Error()
		} else {
			suggested, suggestionNote = u.setupSuggestions(r.Context(), network)
		}
	}
	addresses := LANAddresses()
	selectedAddress := network.IP
	if selectedAddress == "" && len(addresses) != 0 {
		selectedAddress = addresses[0]
	}
	devices := append([]relay.Device(nil), stats.Devices...)
	sort.Slice(devices, func(i, j int) bool { return devices[i].LastSeen.After(devices[j].LastSeen) })
	var latest *relay.Device
	if len(devices) > 0 {
		latest = &devices[0]
	}
	writeJSON(w, map[string]any{
		"heartbeat": u.heartbeat.Snapshot(), "port": cfg.Port, "ui_port": cfg.UIPort,
		"lan_addresses": addresses, "selected_address": selectedAddress,
		"network": network, "network_error": networkError,
		"suggested_ips": suggested, "suggestion_note": suggestionNote,
		"devices": stats.Devices, "latest_device": latest,
	})
}

func (u *UI) setupSuggestions(ctx context.Context, network netcfg.Network) ([]string, string) {
	cacheKey := network.IP + "/" + strconv.Itoa(network.Prefix)
	u.setupMu.Lock()
	defer u.setupMu.Unlock()
	if u.setupKey == cacheKey {
		return append([]string(nil), u.setupIPs...), u.setupNote
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, err := suggestUnusedIPs(probeCtx, network, pingResponds)
	note := "Suggestions are best effort; verify availability with your network administrator."
	if err != nil {
		note = err.Error() + " " + note
	}
	u.setupKey, u.setupIPs, u.setupNote = cacheKey, addresses, note
	return append([]string(nil), addresses...), note
}

func suggestUnusedIPs(ctx context.Context, network netcfg.Network, probe setupProbe) ([]string, error) {
	ip := net.ParseIP(network.IP).To4()
	if ip == nil || network.Prefix < 16 || network.Prefix > 30 {
		return nil, fmt.Errorf("cannot suggest addresses for %s/%d", network.IP, network.Prefix)
	}
	mask := net.CIDRMask(network.Prefix, 32)
	base := ip.Mask(mask).To4()
	baseValue := binary.BigEndian.Uint32(base)
	gateway := net.ParseIP(network.Gateway).To4()
	candidates := make([]string, 0, 3)
	hostCount := 1 << (32 - network.Prefix)
	probes := 0
	for offset := 2; offset < hostCount-1 && len(candidates) < 3 && probes < 64; offset++ {
		candidateBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(candidateBytes, baseValue+uint32(offset))
		candidate := net.IP(candidateBytes)
		if candidate.Equal(ip) || (gateway != nil && candidate.Equal(gateway)) {
			continue
		}
		address := candidate.String()
		probes++
		responded, err := probe(ctx, address)
		if err != nil {
			continue
		}
		if !responded {
			candidates = append(candidates, address)
		}
	}
	return candidates, nil
}

func pingResponds(ctx context.Context, address string) (bool, error) {
	var command string
	var args []string
	if runtime.GOOS == "windows" {
		command, args = "ping.exe", []string{"-n", "1", "-w", "300", address}
	} else {
		command, args = "ping", []string{"-c", "1", "-W", "1", address}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	err := exec.CommandContext(pingCtx, command, args...).Run()
	if err == nil {
		return true, nil
	}
	if pingCtx.Err() != nil {
		return false, nil
	}
	if _, ok := err.(*exec.Error); ok {
		return false, err
	}
	return false, nil
}
