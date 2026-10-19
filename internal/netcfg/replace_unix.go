//go:build !windows

package netcfg

import "os"

func replaceBackup(source, destination string) error { return os.Rename(source, destination) }
