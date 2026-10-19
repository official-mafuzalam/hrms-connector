#!/bin/sh
set -eu

if [ -z "${SIGN_CERT:-}" ] && [ -z "${SIGN_PASSWORD:-}" ]; then
	printf '%s\n' 'WARNING: unsigned HRMS Connector build. Windows SmartScreen may show "Unknown publisher".' >&2
	exit 0
fi
if [ -z "${SIGN_CERT:-}" ] || [ -z "${SIGN_PASSWORD:-}" ]; then
	echo "SIGN_CERT and SIGN_PASSWORD must both be set to sign artifacts." >&2
	exit 1
fi
if ! command -v osslsigncode >/dev/null 2>&1; then
	echo "osslsigncode is required when SIGN_CERT/SIGN_PASSWORD are set." >&2
	exit 1
fi

for artifact do
	temporary="${artifact}.signed"
	rm -f "$temporary"
	osslsigncode sign -pkcs12 "$SIGN_CERT" -pass "$SIGN_PASSWORD" -h sha2 -n "HRMS Connector" -in "$artifact" -out "$temporary"
	mv "$temporary" "$artifact"
done
