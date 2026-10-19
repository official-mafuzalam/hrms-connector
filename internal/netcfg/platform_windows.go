//go:build windows

package netcfg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type windowsDriver struct {
	runner     Runner
	choicePath string
}

func NewPlatform(runner Runner) Driver { return &windowsDriver{runner: runner} }

const powershell = "powershell.exe"

func encodePowerShell(script string) string {
	encoded := make([]byte, 0, len(script)*2)
	for _, char := range script {
		encoded = append(encoded, byte(char), byte(char>>8))
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

func (d *windowsDriver) runScript(ctx context.Context, script string) ([]byte, error) {
	script = "$ProgressPreference='SilentlyContinue'; $ErrorActionPreference='Stop'; " + script
	return d.runner.Run(ctx, powershell, "-NoProfile", "-NonInteractive", "-OutputFormat", "Text", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
}

func (d *windowsDriver) Inspect(ctx context.Context) (Network, error) {
	script := `$ErrorActionPreference='Stop'
$skip='vEthernet|VirtualBox|VMware|Hyper-V|Tailscale|TAP|Loopback'
$options=@(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | ForEach-Object {
  $adapter=Get-NetAdapter -InterfaceIndex $_.InterfaceIndex -ErrorAction SilentlyContinue
  if($null -eq $adapter -or $adapter.Name -match $skip -or $adapter.InterfaceDescription -match $skip){return}
  $addresses=@(Get-NetIPAddress -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4 | Where-Object {$_.IPAddress -notlike '169.254.*' -and $_.IPAddress -notlike '127.*' -and $_.AddressState -eq 'Preferred'})
  if($addresses.Count -eq 0){return}
  $address=$addresses[0]
  $interface=Get-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4
  $dns=@(Get-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4 | ForEach-Object {$_.ServerAddresses} | Where-Object {$_})
  $wifi=($adapter.NdisPhysicalMedium -eq 9 -or $adapter.Name -match 'Wi-?Fi|Wireless')
  [pscustomobject]@{interface=$adapter.Name;ip=$address.IPAddress;prefix=$address.PrefixLength;gateway=$_.NextHop;dns=$dns;dhcp=($interface.Dhcp -eq 'Enabled');wifi=$wifi;physical_ethernet=($adapter.HardwareInterface -and !$wifi);route_metric=[int]$_.RouteMetric;interface_metric=[int]$adapter.InterfaceMetric}
})
ConvertTo-Json -InputObject @($options) -Compress -Depth 5`
	return d.inspectCandidates(ctx, script, nil)
}

func (d *windowsDriver) InspectForDevices(ctx context.Context, deviceIPs []string) (Network, error) {
	script := `$skip='vEthernet|VirtualBox|VMware|Hyper-V|Tailscale|TAP|Loopback'
$options=@(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | ForEach-Object {
  $adapter=Get-NetAdapter -InterfaceIndex $_.InterfaceIndex -ErrorAction SilentlyContinue
  if($null -eq $adapter -or $adapter.Name -match $skip -or $adapter.InterfaceDescription -match $skip){return}
  $addresses=@(Get-NetIPAddress -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4 | Where-Object {$_.IPAddress -notlike '169.254.*' -and $_.IPAddress -notlike '127.*' -and $_.AddressState -eq 'Preferred'})
  if($addresses.Count -eq 0){return}
  $address=$addresses[0]
  $interface=Get-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4
  $dns=@(Get-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4 | ForEach-Object {$_.ServerAddresses} | Where-Object {$_})
  $wifi=($adapter.NdisPhysicalMedium -eq 9 -or $adapter.Name -match 'Wi-?Fi|Wireless')
  [pscustomobject]@{interface=$adapter.Name;ip=$address.IPAddress;prefix=$address.PrefixLength;gateway=$_.NextHop;dns=$dns;dhcp=($interface.Dhcp -eq 'Enabled');wifi=$wifi;physical_ethernet=($adapter.HardwareInterface -and !$wifi);route_metric=[int]$_.RouteMetric;interface_metric=[int]$adapter.InterfaceMetric}
})
ConvertTo-Json -InputObject @($options) -Compress -Depth 5`
	return d.inspectCandidates(ctx, script, deviceIPs)
}

func (d *windowsDriver) inspectCandidates(ctx context.Context, script string, deviceIPs []string) (Network, error) {
	data, err := d.runScript(ctx, script)
	if err != nil {
		return Network{}, err
	}
	var options []AdapterOption
	if err := json.Unmarshal(data, &options); err != nil {
		var single AdapterOption
		if singleErr := json.Unmarshal(data, &single); singleErr != nil {
			return Network{}, fmt.Errorf("parse PowerShell network JSON: %w", err)
		}
		options = []AdapterOption{single}
	}
	if len(options) == 0 {
		return Network{}, errors.New("no eligible default-route adapter was found")
	}
	for i := range options {
		options[i].Metric = effectiveMetric(options[i])
	}
	preferred := d.preferredInterface()
	var selected AdapterOption
	needsChoice := false
	if preferred != "" {
		for _, option := range options {
			if option.Interface == preferred {
				selected = option
				break
			}
		}
	}
	if selected.Interface == "" {
		selected, needsChoice = ChooseAdapter(options, deviceIPs)
	}
	if needsChoice {
		return Network{NeedsChoice: true, Options: options, Note: "Choose the adapter used by the devices; no network changes have been made."}, nil
	}
	mask := net.CIDRMask(selected.Prefix, 32)
	network := Network{
		Interface: selected.Interface, IP: selected.IP, Prefix: selected.Prefix,
		Gateway: selected.Gateway, DNS: selected.DNS, DHCP: selected.DHCP,
		WiFi: selected.WiFi, SubnetMask: net.IP(mask).String(), Options: options,
	}
	return network, nil
}

func (d *windowsDriver) preferredInterface() string {
	if d.choicePath == "" {
		return ""
	}
	data, err := os.ReadFile(d.choicePath)
	if err != nil {
		return ""
	}
	var choice struct {
		Interface string `json:"interface"`
	}
	if json.Unmarshal(data, &choice) != nil {
		return ""
	}
	return choice.Interface
}

func (d *windowsDriver) SelectAdapter(ctx context.Context, name string) error {
	network, err := d.InspectForDevices(ctx, nil)
	if err != nil {
		return err
	}
	found := false
	for _, option := range network.Options {
		if option.Interface == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("adapter %q is not an eligible default-route adapter", name)
	}
	if d.choicePath == "" {
		return errors.New("adapter choice path is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(d.choicePath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Interface string `json:"interface"`
	}{Interface: name})
	if err != nil {
		return fmt.Errorf("encode adapter choice: %w", err)
	}
	return os.WriteFile(d.choicePath, data, 0600)
}

func (d *windowsDriver) SetStatic(ctx context.Context, network Network) error {
	if network.Gateway == "" {
		return errors.New("the adapter must have a gateway")
	}
	data, err := json.Marshal(network)
	if err != nil {
		return err
	}
	payload := base64.StdEncoding.EncodeToString(data)
	mask := net.IP(net.CIDRMask(network.Prefix, 32)).String()
	firstDNS := "none"
	if len(network.DNS) > 0 {
		firstDNS = network.DNS[0]
	}
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$n=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')) | ConvertFrom-Json
& netsh.exe interface ipv4 set address name=$n.interface source=static address=$n.ip mask=%s gateway=$n.gateway gwmetric=1
if($LASTEXITCODE -ne 0){throw 'netsh failed while setting the address'}
& netsh.exe interface ipv4 set dnsservers name=$n.interface source=static address=%s validate=no
if($LASTEXITCODE -ne 0){throw 'netsh failed while setting DNS'}
for($i=1;$i -lt @($n.dns).Count;$i++){
  & netsh.exe interface ipv4 add dnsservers name=$n.interface address=$($n.dns[$i]) index=($i+1) validate=no
  if($LASTEXITCODE -ne 0){throw 'netsh failed while adding DNS'}
}`, payload, mask, firstDNS)
	_, err = d.runScript(ctx, script)
	return err
}

func (d *windowsDriver) SetDHCP(ctx context.Context, network Network) error {
	data, err := json.Marshal(network)
	if err != nil {
		return err
	}
	payload := base64.StdEncoding.EncodeToString(data)
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$n=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')) | ConvertFrom-Json
& netsh.exe interface ipv4 set address name=$n.interface source=dhcp
if($LASTEXITCODE -ne 0){throw 'netsh failed while restoring DHCP address'}
& netsh.exe interface ipv4 set dnsservers name=$n.interface source=dhcp
if($LASTEXITCODE -ne 0){throw 'netsh failed while restoring DHCP DNS'}`, payload)
	_, err = d.runScript(ctx, script)
	return err
}

func (d *windowsDriver) GatewayReachable(ctx context.Context, network Network) bool {
	script := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'; Test-Connection -ComputerName '%s' -Count 1 -Quiet -TimeoutSeconds 1`, strings.ReplaceAll(network.Gateway, "'", "''"))
	output, err := d.runScript(ctx, script)
	return err == nil && strings.EqualFold(strings.TrimSpace(string(output)), "true")
}

func NewSystemManager(dataDir string) *Manager {
	return New(&windowsDriver{runner: ExecRunner{}, choicePath: filepath.Join(dataDir, "adapter-choice.json")}, dataDir)
}
