#!/bin/sh
set -eu

DATA_DIR=/var/lib/hrms-connector
if [ "$(id -u)" -ne 0 ]; then
	echo "Run this uninstaller as root (for example: sudo ./uninstall.sh)." >&2
	exit 1
fi
systemctl disable --now hrms-connector.service 2>/dev/null || true
rm -f /etc/systemd/system/hrms-connector.service
systemctl daemon-reload
if [ -f "$DATA_DIR/netbackup.json" ]; then
	printf 'Restore automatic DHCP addressing? [y/N] '
	IFS= read -r answer
	case "$answer" in
		y|Y|yes|YES) /usr/local/bin/connector netfix --restore --data-dir "$DATA_DIR" ;;
	esac
fi
rm -f /usr/local/bin/connector
echo "HRMS Connector binaries and service were removed. Configuration, logs, and backups remain in $DATA_DIR."
