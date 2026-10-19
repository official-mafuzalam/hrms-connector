//go:build linux

package netcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
)

type linuxRoute struct {
	Device  string `json:"dev"`
	Gateway string `json:"gateway"`
	Metric  int    `json:"metric"`
}

type linuxDriver struct {
	runner       Runner
	netplanCheck func() bool
}

func NewPlatform(runner Runner) Driver {
	return &linuxDriver{runner: runner, netplanCheck: NetplanDetected}
}

func (d *linuxDriver) Inspect(ctx context.Context) (Network, error) {
	routesData, err := d.runner.Run(ctx, "ip", "-j", "route", "show", "default")
	if err != nil {
		return Network{}, err
	}
	var routes []linuxRoute
	if err := json.Unmarshal(routesData, &routes); err != nil {
		return Network{}, fmt.Errorf("parse default route: %w", err)
	}
	routes = filterRoutes(routes)
	if len(routes) != 1 {
		return Network{}, errors.New("could not identify one clear default-route adapter; no changes were made")
	}
	name := routes[0].Device
	deviceData, err := d.runner.Run(ctx, "nmcli", "-t", "-f", "GENERAL.DEVICE,GENERAL.TYPE,GENERAL.CONNECTION,IP4.ADDRESS,IP4.GATEWAY,IP4.DNS,IP4.METHOD", "device", "show", name)
	if err != nil {
		return Network{}, err
	}
	fields := parseNMCLI(string(deviceData))
	ip, prefix, err := net.ParseCIDR(fields["IP4.ADDRESS"])
	if err != nil {
		return Network{}, fmt.Errorf("parse address on %s: %w", name, err)
	}
	ones, _ := prefix.Mask.Size()
	var dns []string
	for _, address := range strings.Split(fields["IP4.DNS"], ",") {
		address = strings.TrimSpace(address)
		if address != "" {
			dns = append(dns, address)
		}
	}
	network := Network{
		Interface: name, Connection: fields["GENERAL.CONNECTION"], IP: ip.String(), Prefix: ones,
		Gateway: fields["IP4.GATEWAY"], DNS: dns,
		DHCP: strings.EqualFold(fields["IP4.METHOD"], "auto"),
		WiFi: strings.EqualFold(fields["GENERAL.TYPE"], "wifi"),
	}
	if network.Connection == "" || network.Connection == "--" {
		network.Connection = name
	}
	if network.Gateway == "" {
		network.Gateway = routes[0].Gateway
	}
	if d.netplanCheck != nil && d.netplanCheck() {
		network.Netplan = true
		network.Note = "netplan is managing network settings; no address was changed. Configure a DHCP reservation on the router, or use an administrator-edited netplan file to set this same IP, prefix, gateway, and DNS."
	}
	return network, nil
}

func filterRoutes(routes []linuxRoute) []linuxRoute {
	filtered := make([]linuxRoute, 0, len(routes))
	for _, route := range routes {
		name := strings.ToLower(route.Device)
		if route.Device == "" || strings.Contains(name, "veth") || strings.Contains(name, "virtualbox") || strings.Contains(name, "vmware") || strings.Contains(name, "tailscale") || strings.Contains(name, "tap") || strings.Contains(name, "tun") || name == "lo" {
			continue
		}
		filtered = append(filtered, route)
	}
	if len(filtered) < 2 {
		return filtered
	}
	best := filtered[:0]
	minimum := filtered[0].Metric
	for _, route := range filtered {
		if route.Metric < minimum {
			minimum = route.Metric
		}
	}
	for _, route := range filtered {
		if route.Metric == minimum {
			best = append(best, route)
		}
	}
	return best
}

func parseNMCLI(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		pair := strings.SplitN(line, ":", 2)
		if len(pair) == 2 {
			key, value := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
			switch key {
			case "IP4.ADDRESS[1]":
				result["IP4.ADDRESS"] = value
			case "IP4.DNS[1]":
				result["IP4.DNS"] = value
			case "IP4.DNS[2]":
				result["IP4.DNS"] += "," + value
			default:
				result[key] = value
			}
		}
	}
	return result
}

func (d *linuxDriver) SetStatic(ctx context.Context, network Network) error {
	address := fmt.Sprintf("%s/%d", network.IP, network.Prefix)
	connection := network.Connection
	args := []string{"connection", "modify", connection, "ipv4.method", "manual", "ipv4.addresses", address, "ipv4.gateway", network.Gateway, "ipv4.dns", strings.Join(network.DNS, ",")}
	if _, err := d.runner.Run(ctx, "nmcli", args...); err != nil {
		return err
	}
	_, err := d.runner.Run(ctx, "nmcli", "connection", "up", connection)
	return err
}

func (d *linuxDriver) SetDHCP(ctx context.Context, network Network) error {
	connection := network.Connection
	if connection == "" {
		connection = network.Interface
	}
	if _, err := d.runner.Run(ctx, "nmcli", "connection", "modify", connection, "ipv4.method", "auto", "ipv4.addresses", "", "ipv4.gateway", "", "ipv4.dns", ""); err != nil {
		return err
	}
	_, err := d.runner.Run(ctx, "nmcli", "connection", "up", connection)
	return err
}

func (d *linuxDriver) GatewayReachable(ctx context.Context, network Network) bool {
	ip := net.ParseIP(network.Gateway)
	if ip == nil {
		return false
	}
	name, args := "ping", []string{"-I", network.Interface, "-c", "1", "-W", "1", network.Gateway}
	if _, err := d.runner.Run(ctx, name, args...); err == nil {
		return true
	}
	return false
}

func NewSystemManager(dataDir string) *Manager {
	return New(NewPlatform(ExecRunner{}), dataDir)
}

func NetplanDetected() bool {
	files, _ := filepath.Glob("/etc/netplan/*.yaml")
	other, _ := filepath.Glob("/etc/netplan/*.yml")
	return len(files)+len(other) > 0
}
