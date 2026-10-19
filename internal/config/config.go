package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"hrms-connector/internal/key"
)

type Config struct {
	Key             string   `json:"key"`
	Port            int      `json:"port"`
	Bind            string   `json:"bind"`
	UIPort          int      `json:"ui_port"`
	Language        string   `json:"language"`
	AllowedCIDRs    []string `json:"allowed_cidrs"`
	UpstreamTimeout int      `json:"upstream_timeout"`
	MaxBodyKB       int      `json:"max_body_kb"`
	TLSVerify       bool     `json:"tls_verify"`
	HTTPProxy       string   `json:"http_proxy"`
	LogLevel        string   `json:"log_level"`
	PINHash         string   `json:"ui_pin_hash"`
}

func Default() Config {
	return Config{
		Port: 17081, Bind: "0.0.0.0", UIPort: 17082, Language: "bn",
		AllowedCIDRs:    []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"},
		UpstreamTimeout: 15, MaxBodyKB: 1024, TLSVerify: true, LogLevel: "info",
	}
}

func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 || c.UIPort < 1 || c.UIPort > 65535 {
		return errors.New("ports must be between 1 and 65535")
	}
	if c.Bind == "" {
		return errors.New("bind address is required")
	}
	if c.Language != "bn" && c.Language != "en" {
		return errors.New("language must be bn or en")
	}
	if c.UpstreamTimeout < 1 || c.UpstreamTimeout > 300 {
		return errors.New("upstream timeout must be between 1 and 300 seconds")
	}
	if c.MaxBodyKB < 1 || c.MaxBodyKB > 102400 {
		return errors.New("maximum body size must be between 1 and 102400 KB")
	}
	if c.LogLevel != "debug" && c.LogLevel != "info" && c.LogLevel != "warn" && c.LogLevel != "error" {
		return errors.New("log level must be debug, info, warn, or error")
	}
	if c.HTTPProxy != "" {
		if _, err := neturl(c.HTTPProxy); err != nil {
			return fmt.Errorf("invalid HTTP proxy: %w", err)
		}
	}
	for _, cidr := range c.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid allowed CIDR %q", cidr)
		}
	}
	if c.Key != "" {
		if _, err := key.Parse(c.Key); err != nil {
			return err
		}
	}
	return nil
}

func neturl(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("must be an HTTP(S) URL with a host")
	}
	return u.Host, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
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
	return replaceFile(name, path)
}

func Path(dataDir string) string { return filepath.Join(dataDir, "config.json") }

func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("PROGRAMDATA")
		if dir == "" {
			dir = `C:\ProgramData`
		}
		return filepath.Join(dir, "HRMS Connector")
	}
	return "/etc/hrms-connector"
}

func (c Config) Timeout() time.Duration { return time.Duration(c.UpstreamTimeout) * time.Second }
