package key

import (
	"encoding/base64"
	"strings"
	"testing"
)

func makeKey(t *testing.T, raw string) string {
	t.Helper()
	return prefix + base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func TestParse(t *testing.T) {
	valid := makeKey(t, `{"v":1,"u":"https://hrms.example/base","t":"token-secret"}`)
	conn, err := Parse(valid)
	if err != nil {
		t.Fatal(err)
	}
	if conn.URL.String() != "https://hrms.example/base" || conn.Token != "token-secret" {
		t.Fatalf("unexpected parsed key: URL=%q token=%q", conn.URL, conn.Token)
	}

	for _, test := range []struct {
		name  string
		value string
	}{
		{"prefix", "HRMS2.abc"},
		{"encoding", prefix + "%%%"},
		{"json", makeKey(t, "{")},
		{"version", makeKey(t, `{"v":2,"u":"https://hrms.example","t":"secret"}`)},
		{"missing token", makeKey(t, `{"v":1,"u":"https://hrms.example","t":""}`)},
		{"plain HTTP", makeKey(t, `{"v":1,"u":"http://hrms.example","t":"secret"}`)},
		{"credentials", makeKey(t, `{"v":1,"u":"https://user:pass@hrms.example","t":"secret"}`)},
		{"query", makeKey(t, `{"v":1,"u":"https://hrms.example/path?q=x","t":"secret"}`)},
		{"invalid URL", makeKey(t, `{"v":1,"u":"://","t":"secret"}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.value); err == nil {
				t.Fatal("expected parsing error")
			}
		})
	}
}

func TestLocalHTTPAndMaskedToken(t *testing.T) {
	value := makeKey(t, `{"v":1,"u":"http://127.0.0.1:8080","t":"abcde-secret"}`)
	if _, err := Parse(value); err != nil {
		t.Fatal(err)
	}
	if masked := Masked(value); masked != "••••cret" {
		t.Fatalf("unexpected mask %q", masked)
	}
	if strings.Contains(Masked(value), "abcde") {
		t.Fatal("masked token reveals token prefix")
	}
}
