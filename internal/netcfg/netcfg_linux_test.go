//go:build linux

package netcfg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type fixtureRunner struct {
	initial        Network
	current        Network
	pingOK         bool
	setStaticCalls int
	setDHCPCalls   int
	routeFixture   []byte
}

func newFixtureRunner(t *testing.T) *fixtureRunner {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "dhcp-network.json"))
	if err != nil {
		t.Fatal(err)
	}
	var network Network
	if err := json.Unmarshal(data, &network); err != nil {
		t.Fatal(err)
	}
	return &fixtureRunner{
		initial: network, current: network, pingOK: true,
		routeFixture: []byte(`[{"dev":"enp1s0","gateway":"192.168.40.1","metric":100}]`),
	}
}

func (r *fixtureRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	switch name {
	case "ip":
		return r.routeFixture, nil
	case "nmcli":
		if strings.Contains(strings.Join(args, " "), "device show") {
			method := "manual"
			if r.current.DHCP {
				method = "auto"
			}
			output := "GENERAL.DEVICE:" + r.current.Interface + "\n" +
				"GENERAL.TYPE:" + map[bool]string{true: "wifi", false: "ethernet"}[r.current.WiFi] + "\n" +
				"GENERAL.CONNECTION:" + r.current.Connection + "\n" +
				"IP4.ADDRESS[1]:" + r.current.IP + "/" + itoa(r.current.Prefix) + "\n" +
				"IP4.GATEWAY:" + r.current.Gateway + "\n" +
				"IP4.DNS[1]:" + r.current.DNS[0] + "\n" +
				"IP4.DNS[2]:" + r.current.DNS[1] + "\n" +
				"IP4.METHOD:" + method + "\n"
			return []byte(output), nil
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "ipv4.method manual"):
			r.setStaticCalls++
			r.current.DHCP = false
		case strings.Contains(joined, "ipv4.method auto"):
			r.setDHCPCalls++
			r.current.DHCP = true
			r.current.IP = r.initial.IP
			r.current.Prefix = r.initial.Prefix
			r.current.Gateway = r.initial.Gateway
			r.current.DNS = append([]string(nil), r.initial.DNS...)
		}
		return nil, nil
	case "ping":
		if r.pingOK {
			return []byte("ok"), nil
		}
		return nil, errors.New("gateway unreachable")
	default:
		return nil, errors.New("unexpected command " + name)
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func testManager(t *testing.T, runner *fixtureRunner) (*Manager, *linuxDriver) {
	t.Helper()
	driver := &linuxDriver{runner: runner, netplanCheck: func() bool { return false }}
	return New(driver, t.TempDir()), driver
}

func TestInspectParsesFixtureJSONAndNMCLIOutput(t *testing.T) {
	runner := newFixtureRunner(t)
	driver := &linuxDriver{runner: runner, netplanCheck: func() bool { return false }}
	got, err := driver.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Interface != "enp1s0" || got.Connection != "Factory LAN" || got.IP != "192.168.40.27" || got.Prefix != 24 || got.Gateway != "192.168.40.1" || !got.DHCP || !reflect.DeepEqual(got.DNS, runner.initial.DNS) {
		t.Fatalf("unexpected parsed network: %+v", got)
	}
}

func TestApplyPreservesAddressAndWritesBackup(t *testing.T) {
	runner := newFixtureRunner(t)
	manager, _ := testManager(t, runner)
	got, err := manager.Apply(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.IP != runner.initial.IP || got.Prefix != runner.initial.Prefix || got.Gateway != runner.initial.Gateway || !reflect.DeepEqual(got.DNS, runner.initial.DNS) || got.DHCP {
		t.Fatalf("static address differs from DHCP lease: %+v", got)
	}
	if runner.setStaticCalls != 1 {
		t.Fatalf("static calls=%d, want 1", runner.setStaticCalls)
	}
	backup, err := readBackup(manager.BackupPath())
	if err != nil {
		t.Fatal(err)
	}
	if !backup.Network.DHCP || backup.Network.IP != runner.initial.IP || backup.Timestamp.IsZero() {
		t.Fatalf("unexpected backup: %+v", backup)
	}
}

func TestStaticAddressIsNoOp(t *testing.T) {
	runner := newFixtureRunner(t)
	runner.current.DHCP = false
	manager, _ := testManager(t, runner)
	got, err := manager.Apply(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.IP != runner.initial.IP || runner.setStaticCalls != 0 {
		t.Fatalf("static address changed: network=%+v calls=%d", got, runner.setStaticCalls)
	}
	if _, err := os.Stat(manager.BackupPath()); !os.IsNotExist(err) {
		t.Fatalf("static no-op wrote backup: %v", err)
	}
}

func TestApplyRefusalsDoNotChangeNetwork(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*fixtureRunner)
		allowWiFi bool
		want      string
	}{
		{"wifi", func(r *fixtureRunner) { r.current.WiFi = true }, false, "Wi-Fi"},
		{"no gateway", func(r *fixtureRunner) {
			r.current.Gateway = ""
			r.routeFixture = []byte(`[{"dev":"enp1s0","metric":100}]`)
		}, false, "no gateway"},
		{"link local", func(r *fixtureRunner) { r.current.IP = "169.254.12.34" }, false, "link-local"},
		{"ambiguous route", func(r *fixtureRunner) {
			r.routeFixture = []byte(`[{"dev":"enp1s0","gateway":"192.168.40.1","metric":10},{"dev":"enp2s0","gateway":"192.168.40.1","metric":10}]`)
		}, false, "one clear"},
		{"wifi allowed", func(r *fixtureRunner) { r.current.WiFi = true }, true, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newFixtureRunner(t)
			test.mutate(runner)
			manager, _ := testManager(t, runner)
			_, err := manager.Apply(context.Background(), test.allowWiFi)
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error=%v, want containing %q", err, test.want)
			}
			if test.want == "" && err != nil {
				t.Fatal(err)
			}
			if test.want != "" && runner.setStaticCalls != 0 {
				t.Fatalf("refusal changed addressing: %d static calls", runner.setStaticCalls)
			}
		})
	}
}

func TestRestoreAutomaticAndAutoRollback(t *testing.T) {
	t.Run("restore backup", func(t *testing.T) {
		runner := newFixtureRunner(t)
		manager, _ := testManager(t, runner)
		if _, err := manager.Apply(context.Background(), false); err != nil {
			t.Fatal(err)
		}

		got, err := manager.Restore(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !got.DHCP || runner.setDHCPCalls != 1 {
			t.Fatalf("DHCP was not restored: %+v", got)
		}
	})
	t.Run("rollback after failed verification", func(t *testing.T) {
		runner := newFixtureRunner(t)
		runner.pingOK = false
		manager, _ := testManager(t, runner)
		_, err := manager.Apply(context.Background(), false)
		if err == nil || !strings.Contains(err.Error(), "rollback completed") {
			t.Fatalf("expected successful automatic rollback, got %v", err)
		}
		if !runner.current.DHCP || runner.setDHCPCalls != 1 {
			t.Fatalf("failed verification did not restore DHCP: %+v", runner.current)
		}
	})
}

func TestChooseAdapterUsesCombinedMetricsAndPhysicalEthernet(t *testing.T) {
	options := []AdapterOption{
		{Interface: "Wi-Fi", IP: "192.168.10.20", Prefix: 24, RouteMetric: 20, InterfaceMetric: 10, WiFi: true},
		{Interface: "Ethernet", IP: "192.168.10.30", Prefix: 24, RouteMetric: 20, InterfaceMetric: 10, PhysicalEthernet: true},
	}
	chosen, needsChoice := ChooseAdapter(options, nil)
	if needsChoice || chosen.Interface != "Ethernet" {
		t.Fatalf("equal effective metrics should prefer physical Ethernet: %+v needsChoice=%v", chosen, needsChoice)
	}
	options[0].InterfaceMetric = 1
	chosen, needsChoice = ChooseAdapter(options, nil)
	if needsChoice || chosen.Interface != "Wi-Fi" {
		t.Fatalf("lower combined metric should win before adapter type: %+v needsChoice=%v", chosen, needsChoice)
	}
}

func TestChooseAdapterUsesDeviceSubnetToResolveMetricTie(t *testing.T) {
	options := []AdapterOption{
		{Interface: "Ethernet A", IP: "192.168.1.10", Prefix: 24, Metric: 10, PhysicalEthernet: true},
		{Interface: "Ethernet B", IP: "10.2.3.10", Prefix: 24, Metric: 10, PhysicalEthernet: true},
	}
	chosen, needsChoice := ChooseAdapter(options, []string{"10.2.3.42"})
	if needsChoice || chosen.Interface != "Ethernet B" {
		t.Fatalf("device subnet did not resolve metric tie: %+v needsChoice=%v", chosen, needsChoice)
	}
}

func TestChooseAdapterUnresolvedTieRequiresChoice(t *testing.T) {
	options := []AdapterOption{
		{Interface: "Ethernet A", IP: "192.168.1.10", Prefix: 24, Metric: 10, PhysicalEthernet: true},
		{Interface: "Ethernet B", IP: "10.2.3.10", Prefix: 24, Metric: 10, PhysicalEthernet: true},
	}
	_, needsChoice := ChooseAdapter(options, []string{"172.16.0.42"})
	if !needsChoice {
		t.Fatal("ambiguous adapter tie did not require a choice")
	}
}

func TestStripCLIXMLErrors(t *testing.T) {
	got := string(StripCLIXMLErrors([]byte("useful output\n#< CLIXML\r\n<Objs>localized error</Objs>")))
	if got != "useful output" {
		t.Fatalf("CLIXML output was not stripped: %q", got)
	}
}
