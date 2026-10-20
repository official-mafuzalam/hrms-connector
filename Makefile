.PHONY: test build-linux build-windows windows installer installer-smoke

VERSION ?= dev
LDFLAGS = -s -w -X hrms-connector/internal/version.Version=$(VERSION)

test:
	go test -race ./...

build-linux:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o dist/connector-linux-amd64 ./cmd/connector

build-windows: windows

windows:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o dist/connector.exe ./cmd/connector
	sh ./scripts/sign-artifacts.sh dist/connector.exe

installer: windows installer-smoke
	makensis -DVERSION=$(VERSION) installer/hrms-connector.nsi
	sh ./scripts/sign-artifacts.sh dist/HRMS-Connector-Setup-$(VERSION).exe

installer-smoke:
	./scripts/check-installer-commands.sh
