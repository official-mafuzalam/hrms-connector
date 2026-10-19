package netcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Network struct {
	Interface   string          `json:"interface"`
	Connection  string          `json:"connection,omitempty"`
	IP          string          `json:"ip"`
	Prefix      int             `json:"prefix"`
	Gateway     string          `json:"gateway"`
	DNS         []string        `json:"dns"`
	DHCP        bool            `json:"dhcp"`
	WiFi        bool            `json:"wifi"`
	Netplan     bool            `json:"netplan,omitempty"`
	Note        string          `json:"note,omitempty"`
	NeedsChoice bool            `json:"needs_choice,omitempty"`
	Options     []AdapterOption `json:"options,omitempty"`
	SubnetMask  string          `json:"subnet_mask,omitempty"`
}

type AdapterOption struct {
	Interface        string   `json:"interface"`
	IP               string   `json:"ip"`
	Prefix           int      `json:"prefix"`
	Gateway          string   `json:"gateway"`
	DNS              []string `json:"dns"`
	DHCP             bool     `json:"dhcp"`
	WiFi             bool     `json:"wifi"`
	PhysicalEthernet bool     `json:"physical_ethernet"`
	Metric           int      `json:"metric"`
	RouteMetric      int      `json:"route_metric"`
	InterfaceMetric  int      `json:"interface_metric"`
}

type Backup struct {
	Timestamp time.Time `json:"timestamp"`
	Network   Network   `json:"network"`
}

type Driver interface {
	Inspect(context.Context) (Network, error)
	SetStatic(context.Context, Network) error
	SetDHCP(context.Context, Network) error
	GatewayReachable(context.Context, Network) bool
}

type Manager struct {
	driver Driver
	path   string
}

func New(driver Driver, dataDir string) *Manager {
	return &Manager{driver: driver, path: filepath.Join(dataDir, "netbackup.json")}
}

func (m *Manager) BackupPath() string { return m.path }

func (m *Manager) Status(ctx context.Context) (Network, error) {
	return m.driver.Inspect(ctx)
}

func (m *Manager) StatusForDevices(ctx context.Context, deviceIPs []string) (Network, error) {
	if driver, ok := m.driver.(interface {
		InspectForDevices(context.Context, []string) (Network, error)
	}); ok {
		return driver.InspectForDevices(ctx, deviceIPs)
	}
	return m.driver.Inspect(ctx)
}

func (m *Manager) SelectAdapter(ctx context.Context, name string) error {
	driver, ok := m.driver.(interface {
		SelectAdapter(context.Context, string) error
	})
	if !ok {
		return errors.New("adapter selection is unavailable on this platform")
	}
	return driver.SelectAdapter(ctx, name)
}

func (m *Manager) Apply(ctx context.Context, allowWiFi bool) (Network, error) {
	network, err := m.driver.Inspect(ctx)
	if err != nil {
		return Network{}, err
	}
	if network.Netplan {
		return network, errors.New(network.Note)
	}
	if network.NeedsChoice {
		return network, errors.New("choose the default-route adapter before changing its addressing mode; no changes were made")
	}
	if !network.DHCP {
		return network, nil
	}
	if network.Interface == "" {
		return network, errors.New("could not identify the adapter holding the default route; no changes were made")
	}
	if network.Gateway == "" {
		return network, errors.New("the default-route adapter has no gateway; no changes were made")
	}
	if network.WiFi && !allowWiFi {
		return network, errors.New("the default-route adapter is Wi-Fi; use --allow-wifi to explicitly allow this change")
	}
	ip := net.ParseIP(network.IP)
	if ip == nil || ip.To4() == nil {
		return network, errors.New("the default-route adapter does not have a valid IPv4 address; no changes were made")
	}
	if ip.IsLinkLocalUnicast() {
		return network, errors.New("the adapter has a link-local 169.254.x.x address; no changes were made")
	}
	if network.Prefix < 1 || network.Prefix > 32 {
		return network, errors.New("the adapter has an invalid IPv4 prefix; no changes were made")
	}
	backup := Backup{Timestamp: time.Now().UTC(), Network: network}
	if err := saveBackup(m.path, backup); err != nil {
		return network, fmt.Errorf("save network backup: %w", err)
	}
	if err := m.driver.SetStatic(ctx, network); err != nil {
		restoreErr := m.driver.SetDHCP(context.WithoutCancel(ctx), network)
		if restoreErr != nil {
			return network, fmt.Errorf("set static address: %v; automatic rollback failed: %w", err, restoreErr)
		}
		return network, fmt.Errorf("set static address: %w (automatic DHCP rollback completed)", err)
	}
	verified, inspectErr := m.driver.Inspect(ctx)
	if inspectErr == nil && (verified.IP != network.IP || verified.Prefix != network.Prefix || verified.Gateway != network.Gateway || !sameDNS(verified.DNS, network.DNS) || verified.DHCP || !m.driver.GatewayReachable(ctx, network)) {
		inspectErr = errors.New("static settings verification failed: address or gateway did not remain reachable")
	}
	if inspectErr != nil {
		restoreErr := m.driver.SetDHCP(context.WithoutCancel(ctx), network)
		if restoreErr != nil {
			return network, fmt.Errorf("%v; automatic DHCP rollback failed: %w", inspectErr, restoreErr)
		}
		return network, fmt.Errorf("%v; automatic DHCP rollback completed", inspectErr)
	}
	return verified, nil
}

func sameDNS(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ChooseAdapter(options []AdapterOption, deviceIPs []string) (AdapterOption, bool) {
	if len(options) == 0 {
		return AdapterOption{}, true
	}
	bestMetric := effectiveMetric(options[0])
	for _, option := range options[1:] {
		metric := effectiveMetric(option)
		if metric < bestMetric {
			bestMetric = metric
		}
	}
	candidates := make([]AdapterOption, 0, len(options))
	for _, option := range options {
		if effectiveMetric(option) == bestMetric {
			candidates = append(candidates, option)
		}
	}
	matching := make([]AdapterOption, 0, len(candidates))
	for _, option := range candidates {
		if containsDeviceSubnet(option, deviceIPs) {
			matching = append(matching, option)
		}
	}
	if len(matching) > 0 {
		candidates = matching
	}
	ethernet := make([]AdapterOption, 0, len(candidates))
	for _, option := range candidates {
		if option.PhysicalEthernet && !option.WiFi {
			ethernet = append(ethernet, option)
		}
	}
	if len(ethernet) > 0 {
		candidates = ethernet
	}
	if len(candidates) != 1 {
		return AdapterOption{}, true
	}
	return candidates[0], false
}

func effectiveMetric(option AdapterOption) int {
	if option.RouteMetric != 0 || option.InterfaceMetric != 0 {
		return option.RouteMetric + option.InterfaceMetric
	}
	return option.Metric
}

func containsDeviceSubnet(option AdapterOption, deviceIPs []string) bool {
	ip := net.ParseIP(option.IP).To4()
	if ip == nil || option.Prefix < 0 || option.Prefix > 32 {
		return false
	}
	mask := net.CIDRMask(option.Prefix, 32)
	network := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
	for _, deviceIP := range deviceIPs {
		if parsed := net.ParseIP(deviceIP).To4(); parsed != nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

func (m *Manager) Restore(ctx context.Context) (Network, error) {
	backup, err := readBackup(m.path)
	if err != nil {
		return Network{}, err
	}
	if err := m.driver.SetDHCP(ctx, backup.Network); err != nil {
		return Network{}, fmt.Errorf("restore automatic addressing: %w", err)
	}
	network, err := m.driver.Inspect(ctx)
	if err != nil {
		return Network{}, err
	}
	if !network.DHCP {
		return network, errors.New("automatic addressing restoration could not be verified")
	}
	return network, nil
}

func saveBackup(path string, backup Backup) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".netbackup-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceBackup(name, path)
}

func readBackup(path string) (Backup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Backup{}, fmt.Errorf("network backup is unavailable: %w", err)
	}
	var backup Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		return Backup{}, fmt.Errorf("parse network backup: %w", err)
	}
	if backup.Network.Interface == "" || !backup.Network.DHCP {
		return Backup{}, errors.New("network backup is invalid or was not created from DHCP")
	}
	return backup, nil
}
