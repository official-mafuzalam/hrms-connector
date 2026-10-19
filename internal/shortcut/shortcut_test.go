package shortcut

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteShortcutContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "HRMS Connector.url")
	address := "http://127.0.0.1:17082/"
	icon := `C:\Program Files\HRMS Connector\connector.exe`
	if err := Write(path, address, icon); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "[InternetShortcut]\r\nURL=" + address + "\r\nIconFile=" + icon + "\r\nIconIndex=0\r\n"
	if string(data) != want {
		t.Fatalf("shortcut content:\n%s\nwant:\n%s", data, want)
	}
}
