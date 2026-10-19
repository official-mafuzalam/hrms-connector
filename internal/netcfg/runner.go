package netcfg

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	output = StripCLIXMLErrors(output)
	if err != nil {
		return output, fmt.Errorf("%s failed: %w: %s", name, err, string(output))
	}
	return output, nil
}

func StripCLIXMLErrors(output []byte) []byte {
	text := string(output)
	if index := strings.Index(text, "#< CLIXML"); index >= 0 {
		text = text[:index]
	}
	return []byte(strings.TrimSpace(text))
}
