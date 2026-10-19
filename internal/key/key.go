package key

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const prefix = "HRMS1."

var ErrInvalid = errors.New("invalid connection key")

type Connection struct {
	URL   *url.URL
	Token string
}

type payload struct {
	Version int    `json:"v"`
	URL     string `json:"u"`
	Token   string `json:"t"`
}

func Parse(value string) (*Connection, error) {
	if !strings.HasPrefix(value, prefix) {
		return nil, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, fmt.Errorf("%w: malformed encoding", ErrInvalid)
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil || p.Version != 1 || p.Token == "" {
		return nil, fmt.Errorf("%w: malformed payload", ErrInvalid)
	}
	u, err := url.Parse(p.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%w: malformed server URL", ErrInvalid)
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLocal(u.Hostname())) {
		return nil, fmt.Errorf("%w: HTTPS is required", ErrInvalid)
	}
	if u.Scheme != "https" && !isLocal(u.Hostname()) {
		return nil, fmt.Errorf("%w: HTTP is allowed only for localhost", ErrInvalid)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return &Connection{URL: u, Token: p.Token}, nil
}

func isLocal(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func Masked(value string) string {
	conn, err := Parse(value)
	if err != nil {
		return ""
	}
	token := conn.Token
	if len(token) > 4 {
		token = token[len(token)-4:]
	}
	return "••••" + token
}
