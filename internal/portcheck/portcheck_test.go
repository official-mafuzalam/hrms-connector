package portcheck

import (
	"context"
	"net"
	"strings"
	"testing"
)

type runnerValue struct {
	output []byte
	err    error
}

func (r runnerValue) Run(context.Context, string, ...string) ([]byte, error) {
	return r.output, r.err
}

func TestParseExcludedRangeFixture(t *testing.T) {
	fixture := `Protocol tcp Port Exclusion Ranges
Start Port    End Port
----------    --------
      50000       50099
      17082       17082     *
`
	ranges := ParseExcludedRanges(fixture)
	if len(ranges) != 2 || ranges[0] != [2]int{50000, 50099} || ranges[1] != [2]int{17082, 17082} {
		t.Fatalf("parsed ranges = %v", ranges)
	}
}

func TestCheckDetectsBindConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	result := Check(context.Background(), runnerValue{}, "127.0.0.1", port)
	if result.Available || !strings.Contains(result.Reason, "cannot be bound") {
		t.Fatalf("expected bind conflict, got %+v", result)
	}
}
