package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
)

func Content(address, icon string) string {
	return fmt.Sprintf("[InternetShortcut]\r\nURL=%s\r\nIconFile=%s\r\nIconIndex=0\r\n", address, icon)
}

func Write(path, address, icon string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Content(address, icon)), 0600)
}
