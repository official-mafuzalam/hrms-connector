#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BIN=${CONNECTOR_BIN:-"$SCRIPT_DIR/dist/connector-linux-amd64"}
DATA_DIR=/var/lib/hrms-connector

if [ "$(id -u)" -ne 0 ]; then
	echo "Run this installer as root (for example: sudo ./install.sh)." >&2
	exit 1
fi
if [ ! -x "$BIN" ]; then
	echo "Connector binary not found: $BIN. Build it with make build-linux first." >&2
	exit 1
fi

printf 'HRMS Connection Key: '
stty -echo
IFS= read -r CONNECTION_KEY
stty echo
printf '\n'
if [ -z "$CONNECTION_KEY" ]; then
	echo "A connection key is required." >&2
	exit 1
fi
"$BIN" verify-key "$CONNECTION_KEY"

if [ -x "$(command -v nmcli || true)" ]; then
	printf 'Try to keep the current IP by making its DHCP address static? [y/N] '
	IFS= read -r answer
	case "$answer" in
		y|Y|yes|YES)
			"$BIN" netfix --apply --data-dir "$DATA_DIR" || echo "Network address was not changed. Review the netfix message above."
			;;
	esac
else
	echo "NetworkManager/nmcli is unavailable; automatic address fixing is not supported here."
fi

install -d -o root -g root -m 0755 /usr/local/bin
install -o root -g root -m 0755 "$BIN" /usr/local/bin/connector
if ! getent group hrms-connector >/dev/null 2>&1; then
	groupadd --system hrms-connector
fi
if ! id hrms-connector >/dev/null 2>&1; then
	useradd --system --gid hrms-connector --home-dir "$DATA_DIR" --shell /usr/sbin/nologin hrms-connector
fi
install -d -o hrms-connector -g hrms-connector -m 0750 "$DATA_DIR"
/usr/local/bin/connector configure --key "$CONNECTION_KEY" --data-dir "$DATA_DIR"
install -o root -g root -m 0644 "$SCRIPT_DIR/linux/hrms-connector.service" /etc/systemd/system/hrms-connector.service
systemctl daemon-reload
systemctl enable --now hrms-connector.service
UI_PORT=$(/usr/local/bin/connector config-port --kind ui --data-dir "$DATA_DIR")
echo "Installed and started HRMS Connector. Local UI: http://127.0.0.1:$UI_PORT"
