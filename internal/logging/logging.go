package logging

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	*log.Logger
	path string
}

func New(dataDir string) (*Logger, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "connector.log")
	writer := &lumberjack.Logger{Filename: path, MaxSize: 5, MaxBackups: 3, Compress: true}
	return &Logger{Logger: log.New(writer, "", 0), path: path}, nil
}

func (l *Logger) Tail(limit int) ([]string, error) {
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return lines, nil
}

func (l *Logger) Path() string { return l.path }
