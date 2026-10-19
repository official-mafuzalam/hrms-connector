package portcheck

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type Result struct {
	Port      int    `json:"port"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func Check(ctx context.Context, runner Runner, address string, port int) Result {
	result := Result{Port: port}
	listener, err := net.Listen("tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		result.Reason = fmt.Sprintf("port cannot be bound: %v", err)
		return result
	}
	if err := listener.Close(); err != nil {
		result.Reason = fmt.Sprintf("release port check: %v", err)
		return result
	}
	if runtime.GOOS == "windows" {
		output, err := runner.Run(ctx, "netsh.exe", "int", "ipv4", "show", "excludedportrange", "protocol=tcp")
		if err != nil {
			result.Reason = fmt.Sprintf("check Windows excluded TCP ports: %v: %s", err, strings.TrimSpace(string(output)))
			return result
		}
		for _, interval := range ParseExcludedRanges(string(output)) {
			if port >= interval[0] && port <= interval[1] {
				result.Reason = fmt.Sprintf("port is excluded by Windows (%d-%d)", interval[0], interval[1])
				return result
			}
		}
	}
	result.Available = true
	return result
}

var portRangePattern = regexp.MustCompile(`(?m)^\s*(\d+)\s+(\d+)(?:\s+\*)?\s*$`)

func ParseExcludedRanges(output string) [][2]int {
	matches := portRangePattern.FindAllStringSubmatch(output, -1)
	ranges := make([][2]int, 0, len(matches))
	for _, match := range matches {
		start, errStart := strconv.Atoi(match[1])
		end, errEnd := strconv.Atoi(match[2])
		if errStart == nil && errEnd == nil && start >= 1 && end >= start && end <= 65535 {
			ranges = append(ranges, [2]int{start, end})
		}
	}
	return ranges
}
