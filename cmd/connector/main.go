package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"hrms-connector/internal/config"
	"hrms-connector/internal/heartbeat"
	"hrms-connector/internal/key"
	"hrms-connector/internal/logging"
	"hrms-connector/internal/netcfg"
	"hrms-connector/internal/portcheck"
	"hrms-connector/internal/relay"
	"hrms-connector/internal/shortcut"
	"hrms-connector/internal/version"
	"hrms-connector/internal/webui"
)

func main() {
	if err := execute(os.Args[1:]); err != nil {
		var verificationError *heartbeat.VerificationError
		if errors.As(err, &verificationError) && len(os.Args) > 1 && os.Args[1] == "verify-key" {
			fmt.Fprintln(os.Stdout, verificationError.DisplayBilingual())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func execute(args []string) error {
	if len(args) == 0 {
		return openLocalUI(config.DefaultDataDir())
	}
	switch args[0] {
	case "version":
		fmt.Println(version.Version)
		return nil
	case "verify-key":
		if len(args) != 2 {
			return errors.New("usage: connector verify-key <key>")
		}
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout())
		defer cancel()
		status, err := heartbeat.VerifyKey(ctx, cfg, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Connection key verified\nCompany: %s\nSite: %s\n", status.Company, status.Site)
		return nil
	case "netfix":
		return runNetfix(args[1:])
	case "wait-ready":
		return waitReady(args[1:])
	case "check-ports":
		return checkPorts(args[1:])
	case "check-port":
		return checkPort(args[1:])
	case "allocate-port":
		return allocatePort(args[1:])
	case "config-port":
		return showConfigPort(args[1:])
	case "shortcut":
		return writeShortcut(args[1:])
	case "configure":
		return configure(args[1:])
	case "run":
		flags := flag.NewFlagSet("run", flag.ContinueOnError)
		dataDir := flags.String("data-dir", "", "configuration and log directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("run does not accept positional arguments")
		}
		dir := *dataDir
		if dir == "" {
			dir = config.DefaultDataDir()
		}
		return runAsService(dir)
	case "install", "uninstall", "start", "stop":
		if len(args) != 1 {
			return fmt.Errorf("%s takes no options", args[0])
		}
		return controlService(args[0])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func openLocalUI(dataDir string) error {
	cfg, err := config.Load(config.Path(dataDir))
	if os.IsNotExist(err) {
		cfg = config.Default()
	} else if err != nil {
		return fmt.Errorf("load connector configuration: %w", err)
	}
	address := fmt.Sprintf("http://127.0.0.1:%d/", cfg.UIPort)
	if cfg.Key == "" {
		address = fmt.Sprintf("http://127.0.0.1:%d/setup", cfg.UIPort)
	}
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "rundll32.exe", []string{"url.dll,FileProtocolHandler", address}
	case "darwin":
		name, args = "open", []string{address}
	default:
		name, args = "xdg-open", []string{address}
	}
	if err := exec.Command(name, args...).Start(); err != nil {
		return fmt.Errorf("open HRMS Connector in the default browser: %w", err)
	}
	return nil
}

func waitReady(args []string) error {
	dir, timeout, err := parseWaitReadyArgs(args)
	if err != nil {
		return err
	}
	if dir == "" {
		dir = config.DefaultDataDir()
	}
	cfg, err := config.Load(config.Path(dir))
	if err != nil {
		return err
	}
	address := fmt.Sprintf("http://127.0.0.1:%d/api/status", cfg.UIPort)
	return waitForURL(address, timeout)
}

func parseWaitReadyArgs(args []string) (string, time.Duration, error) {
	flags := flag.NewFlagSet("wait-ready", flag.ContinueOnError)
	timeout := 30 * time.Second
	flags.Var(&waitTimeoutValue{duration: &timeout}, "timeout", "maximum time to wait for the local UI (duration or integer seconds)")
	dataDir := flags.String("data-dir", "", "configuration directory")
	if err := flags.Parse(args); err != nil {
		return "", 0, err
	}
	if flags.NArg() != 0 || timeout <= 0 {
		return "", 0, errors.New("usage: connector wait-ready [--timeout 30|30s] [--data-dir PATH]")
	}
	return *dataDir, timeout, nil
}

type waitTimeoutValue struct {
	duration *time.Duration
}

func (v *waitTimeoutValue) String() string {
	if v.duration == nil {
		return ""
	}
	return v.duration.String()
}

func (v *waitTimeoutValue) Set(value string) error {
	duration, err := parseWaitTimeout(value)
	if err != nil {
		return err
	}
	*v.duration = duration
	return nil
}

func parseWaitTimeout(value string) (time.Duration, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return duration, nil
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("timeout must be a duration (for example 30s) or integer seconds: %q", value)
	}
	maxSeconds := int64((1<<63 - 1) / int64(time.Second))
	minSeconds := int64(-maxSeconds)
	if int64(seconds) > maxSeconds || int64(seconds) < minSeconds {
		return 0, fmt.Errorf("timeout seconds are out of range: %q", value)
	}
	return time.Duration(seconds) * time.Second, nil
}

func waitForURL(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(http.MethodGet, address, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("UI returned HTTP %d", response.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("no response received")
	}
	return fmt.Errorf("HRMS Connector UI did not become ready within %s: %w", timeout, lastErr)
}

func checkPorts(args []string) error {
	flags := flag.NewFlagSet("check-ports", flag.ContinueOnError)
	relayPort := flags.Int("relay", 17081, "relay TCP port; use 0 to skip")
	uiPort := flags.Int("ui", 17082, "local UI TCP port; use 0 to skip")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("check-ports does not accept positional arguments")
	}
	if *relayPort == 0 && *uiPort == 0 {
		return errors.New("check-ports requires --relay and/or --ui")
	}
	if (*relayPort != 0 && (*relayPort < 1024 || *relayPort > 49151)) ||
		(*uiPort != 0 && (*uiPort < 1024 || *uiPort > 49151)) {
		return errors.New("new relay and UI ports must be between 1024 and 49151")
	}
	if *relayPort != 0 && *uiPort != 0 && *relayPort == *uiPort {
		return errors.New("relay and UI ports must be different")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runner := portcheck.ExecRunner{}
	relayResult := portcheck.Result{Port: *relayPort, Available: true}
	uiResult := portcheck.Result{Port: *uiPort, Available: true}
	if *relayPort != 0 {
		relayResult = portcheck.Check(ctx, runner, "0.0.0.0", *relayPort)
	}
	if *uiPort != 0 {
		uiResult = portcheck.Check(ctx, runner, "127.0.0.1", *uiPort)
	}
	result := struct {
		Relay portcheck.Result `json:"relay"`
		UI    portcheck.Result `json:"ui"`
	}{Relay: relayResult, UI: uiResult}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return err
	}
	if !relayResult.Available || !uiResult.Available {
		return errors.New("one or more ports are unavailable")
	}
	return nil
}

func checkPort(args []string) error {
	if len(args) != 1 || !decimalDigits(args[0]) {
		return errors.New("usage: connector check-port <port> (digits only, 1024-49151)")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return errors.New("port must contain decimal digits only")
	}
	return checkPortWith(port, portcheck.ExecRunner{}, "0.0.0.0")
}

func checkPortWith(port int, runner portcheck.Runner, address string) error {
	if port < 1024 || port > 49151 {
		return errors.New("port must be between 1024 and 49151")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := portcheck.Check(ctx, runner, address, port)
	if !result.Available {
		return fmt.Errorf("port %d is unavailable: %s", port, result.Reason)
	}
	fmt.Printf("Port %d is available.\n", port)
	return nil
}

func allocatePort(args []string) error {
	flags := flag.NewFlagSet("allocate-port", flag.ContinueOnError)
	preferred := flags.Int("preferred", 0, "preferred port, or 0 to select one")
	excluded := flags.Int("exclude", 0, "port that must not be selected")
	bind := flags.String("bind", "0.0.0.0", "address to check: 0.0.0.0 or 127.0.0.1")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("allocate-port does not accept positional arguments")
	}
	if (*preferred != 0 && (*preferred < 1024 || *preferred > 49151)) ||
		(*excluded != 0 && (*excluded < 1024 || *excluded > 49151)) {
		return errors.New("ports must be between 1024 and 49151")
	}
	if *bind != "0.0.0.0" && *bind != "127.0.0.1" {
		return errors.New("bind must be 0.0.0.0 or 127.0.0.1")
	}
	port, err := findAvailablePort(*preferred, *excluded, *bind, portcheck.ExecRunner{})
	if err != nil {
		return err
	}
	fmt.Print(port)
	return nil
}

func findAvailablePort(preferred, excluded int, bind string, runner portcheck.Runner) (int, error) {
	check := func(port int) bool {
		if port == excluded {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return portcheck.Check(ctx, runner, bind, port).Available
	}
	if preferred != 0 && check(preferred) {
		return preferred, nil
	}

	const firstPort, lastPort = 1024, 49151
	count := int64(lastPort - firstPort + 1)
	offset, err := rand.Int(rand.Reader, big.NewInt(count))
	if err != nil {
		return 0, fmt.Errorf("choose an available port: %w", err)
	}
	start := firstPort + int(offset.Int64())
	for i := 0; i < int(count); i++ {
		port := firstPort + (start-firstPort+i)%int(count)
		if check(port) {
			return port, nil
		}
	}
	return 0, errors.New("no available port found between 1024 and 49151")
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func showConfigPort(args []string) error {
	flags := flag.NewFlagSet("config-port", flag.ContinueOnError)
	dataDir := flags.String("data-dir", "", "configuration directory")
	kind := flags.String("kind", "", "relay or ui")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("config-port does not accept positional arguments")
	}
	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}
	cfg, err := config.Load(config.Path(dir))
	if err != nil {
		return err
	}
	switch *kind {
	case "relay":
		fmt.Print(cfg.Port)
	case "ui":
		fmt.Print(cfg.UIPort)
	default:
		return errors.New("config-port requires --kind relay or --kind ui")
	}
	return nil
}

func writeShortcut(args []string) error {
	flags := flag.NewFlagSet("shortcut", flag.ContinueOnError)
	path := flags.String("path", "", "shortcut file path")
	address := flags.String("url", "", "local connector URL")
	icon := flags.String("icon", "", "application icon path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *path == "" || *address == "" || *icon == "" {
		return errors.New("usage: connector shortcut --path PATH --url URL --icon PATH")
	}
	return shortcut.Write(*path, *address, *icon)
}

func configure(args []string) error {
	flags := flag.NewFlagSet("configure", flag.ContinueOnError)
	connectionKey := flags.String("key", "", "HRMS connection key")
	port := flags.Int("port", 0, "device relay port")
	uiPort := flags.Int("ui-port", 0, "local UI port")
	dataDir := flags.String("data-dir", "", "configuration directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("configure does not accept positional arguments")
	}
	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}
	path := config.Path(dir)
	cfg, err := config.Load(path)
	if os.IsNotExist(err) {
		cfg = config.Default()
	} else if err != nil {
		return err
	}
	if *connectionKey != "" {
		if _, err := key.Parse(*connectionKey); err != nil {
			return err
		}
		cfg.Key = *connectionKey
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if *uiPort != 0 {
		cfg.UIPort = *uiPort
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	fmt.Println("Configuration saved.")
	return nil
}

func runNetfix(args []string) error {
	flags := flag.NewFlagSet("netfix", flag.ContinueOnError)
	statusFlag := flags.Bool("status", false, "show the default-route adapter's addressing mode")
	applyFlag := flags.Bool("apply", false, "keep the current address by making it static")
	restoreFlag := flags.Bool("restore", false, "restore automatic DHCP addressing")
	allowWiFi := flags.Bool("allow-wifi", false, "allow applying to a Wi-Fi adapter")
	jsonOutput := flags.Bool("json", false, "print JSON output")
	dataDir := flags.String("data-dir", "", "connector data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("netfix does not accept positional arguments")
	}
	selected := 0
	for _, enabled := range []bool{*statusFlag, *applyFlag, *restoreFlag} {
		if enabled {
			selected++
		}
	}
	if selected > 1 {
		return errors.New("choose only one of --status, --apply, or --restore")
	}
	if selected == 0 {
		*statusFlag = true
	}
	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}
	manager := netcfg.NewSystemManager(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var network netcfg.Network
	var err error
	switch {
	case *applyFlag:
		network, err = manager.Apply(ctx, *allowWiFi)
	case *restoreFlag:
		network, err = manager.Restore(ctx)
	default:
		network, err = manager.Status(ctx)
	}
	if *jsonOutput {
		result := struct {
			Network netcfg.Network `json:"network"`
			Error   string         `json:"error,omitempty"`
		}{Network: network}
		if err != nil {
			result.Error = err.Error()
		}
		if encodeErr := json.NewEncoder(os.Stdout).Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	}
	if network.Interface != "" {
		mode := "Fixed"
		if network.DHCP {
			mode = "Automatic (may change)"
		}
		fmt.Printf("%s: %s · %s/%d · gateway %s\n", network.Interface, mode, network.IP, network.Prefix, network.Gateway)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	switch {
	case *applyFlag:
		if network.DHCP {
			fmt.Println("Address configuration was set to static without changing the IP.")
		} else {
			fmt.Println("Address is already fixed; no changes were made.")
		}
	case *restoreFlag:
		fmt.Println("Automatic DHCP addressing restored.")
	}
	return nil
}

type serviceProgram struct {
	dataDir string
	cancel  context.CancelFunc
	done    chan error
}

func (p *serviceProgram) Start(service.Service) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	p.cancel, p.done = cancel, make(chan error, 1)
	go func() { p.done <- Run(ctx, p.dataDir) }()
	return nil
}

func (p *serviceProgram) Stop(service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done == nil {
		return nil
	}
	select {
	case err := <-p.done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("connector shutdown timed out")
	}
}

func serviceInstance(dataDir string) (service.Service, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	options := service.KeyValue{
		"StartType":              "automatic",
		"DelayedAutoStart":       true,
		"OnFailure":              "restart",
		"OnFailureDelayDuration": "5s",
		"OnFailureResetPeriod":   86400,
		"LogOutput":              false,
	}
	if runtime.GOOS == "windows" {
		return service.New(&serviceProgram{dataDir: dataDir}, &service.Config{
			Name: "HRMSConnector", DisplayName: "HRMS Connector",
			Description: "Forwards ZKTeco ADMS attendance data to the HRMS server.",
			UserName:    `NT AUTHORITY\LocalService`, Executable: executable,
			Arguments: []string{"run"}, Option: options,
		})
	}
	return service.New(&serviceProgram{dataDir: dataDir}, &service.Config{
		Name: "hrms-connector", DisplayName: "HRMS Connector",
		Description: "Forwards ZKTeco ADMS attendance data to the HRMS server.",
		Executable:  executable, Arguments: []string{"run"}, Option: options,
	})
}

func runAsService(dataDir string) error {
	if dataDir == "" {
		dataDir = config.DefaultDataDir()
	}
	instance, err := serviceInstance(dataDir)
	if err != nil {
		return err
	}
	return instance.Run()
}

func controlService(action string) error {
	instance, err := serviceInstance(config.DefaultDataDir())
	if err != nil {
		return err
	}
	return service.Control(instance, action)
}

func Run(ctx context.Context, dataDir string) error {
	if dataDir == "" {
		dataDir = config.DefaultDataDir()
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	configPath := config.Path(dataDir)
	cfg, err := config.Load(configPath)
	if os.IsNotExist(err) {
		cfg = config.Default()
		if err := config.Save(configPath, cfg); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	} else if err != nil {
		return err
	}
	logger, err := logging.New(dataDir)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	store := config.NewStore(configPath, cfg)
	proxy := relay.New(store, logger)
	hb := heartbeat.New(store, proxy)
	health := &relayListenerHealth{}
	health.set(false, "Relay listener is starting.")
	ui := webui.New(store, hb, proxy, logger, dataDir, netcfg.NewSystemManager(dataDir))
	ui.SetListenerHealth(health)
	var workers sync.WaitGroup
	workers.Add(3)
	go func() {
		defer workers.Done()
		serveRelayLoop(ctx, store, proxy, logger, health)
	}()
	go func() {
		defer workers.Done()
		serveUILoop(ctx, cfg.UIPort, ui, logger)
	}()
	go func() {
		defer workers.Done()
		hb.Run(ctx)
	}()
	logger.Printf("HRMS Connector %s started; relay=%s:%d UI=127.0.0.1:%d", version.Version, cfg.Bind, cfg.Port, cfg.UIPort)
	<-ctx.Done()
	workers.Wait()
	return nil
}

func openRelay(cfg config.Config, handler http.Handler) (net.Listener, *http.Server, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port)))
	if err != nil {
		return nil, nil, fmt.Errorf("listen for device relay on %s:%d: %w", cfg.Bind, cfg.Port, err)
	}
	return listener, &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}, nil
}

type relayListenerHealth struct {
	mu      sync.RWMutex
	healthy bool
	error   string
}

func (h *relayListenerHealth) Snapshot() (bool, string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy, h.error
}

func (h *relayListenerHealth) set(healthy bool, message string) {
	h.mu.Lock()
	h.healthy, h.error = healthy, message
	h.mu.Unlock()
}

func serveRelayLoop(ctx context.Context, store *config.Store, handler http.Handler, logger *logging.Logger, health *relayListenerHealth) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var activeServer *http.Server
	var activeBind string
	var activePort int
	var serverErrors chan error
	for {
		cfg := store.Snapshot()
		if activeServer != nil && (cfg.Bind != activeBind || cfg.Port != activePort) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := activeServer.Shutdown(shutdownCtx); err != nil {
				logger.Printf("old relay listener shutdown failed: %v", err)
			}
			cancel()
			activeServer = nil
			health.set(false, "Relay listener is changing to the configured port.")
		}
		if activeServer == nil {
			listener, server, err := openRelay(cfg, handler)
			if err != nil {
				message := fmt.Sprintf("Relay unavailable on %s:%d: %v", cfg.Bind, cfg.Port, err)
				logger.Printf("%s; retrying", message)
				health.set(false, message)
			} else {
				activeServer, activeBind, activePort = server, cfg.Bind, cfg.Port
				serverErrors = make(chan error, 1)
				go func(server *http.Server, listener net.Listener, serveErrors chan<- error) {
					if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
						serveErrors <- err
					}
				}(server, listener, serverErrors)
				health.set(true, "")
				logger.Printf("relay listener ready on %s:%d", cfg.Bind, cfg.Port)
			}
		}
		select {
		case <-ctx.Done():
			if activeServer != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := activeServer.Shutdown(shutdownCtx); err != nil {
					logger.Printf("relay shutdown failed: %v", err)
				}
				cancel()
			}
			return
		case err := <-serverErrors:
			message := "Relay server stopped: " + err.Error()
			logger.Printf("%s; retrying", message)
			health.set(false, message)
			activeServer = nil
		case <-ticker.C:
		}
	}
}

func serveUILoop(ctx context.Context, port int, handler http.Handler, logger *logging.Logger) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			logger.Printf("local UI unavailable on 127.0.0.1:%d: %v; retrying", port, err)
		} else {
			server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
			serveErr := make(chan error, 1)
			go func() { serveErr <- server.Serve(listener) }()
			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := server.Shutdown(shutdownCtx); err != nil {
					logger.Printf("local UI shutdown failed: %v", err)
				}
				cancel()
				return
			case err := <-serveErr:
				if !errors.Is(err, http.ErrServerClosed) {
					logger.Printf("local UI stopped: %v; retrying", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
