#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM

go build -o "$TEMP_DIR/connector" "$ROOT/cmd/connector"

grep -nE 'connector(-verify)?\.exe"' "$ROOT/installer/hrms-connector.nsi"

python3 - "$ROOT/installer/hrms-connector.nsi" "$TEMP_DIR/connector" <<'PY'
import re
import shlex
import subprocess
import sys
from pathlib import Path

installer = Path(sys.argv[1])
connector = sys.argv[2]
text = installer.read_text(encoding="utf-8")
pattern = re.compile(r"""connector(?:-verify)?\.exe"\s+([^']+)'""")
invocations = []

for line_number, line in enumerate(text.splitlines(), 1):
    if "connector.exe\"" not in line and "connector-verify.exe\"" not in line:
        continue
    match = pattern.search(line)
    if match:
        invocations.append((line_number, match.group(1).strip()))

if not invocations:
    raise SystemExit("No connector.exe invocations found in installer script")

flag_cache = {}
for line_number, command_line in invocations:
    command_line = command_line.replace(
        "$ConfigArgs",
        'configure --port 8085 --ui-port 8086 --data-dir "C:\\ProgramData\\HRMS Connector" --key "HRMS1.sample"',
    )
    try:
        words = shlex.split(command_line, posix=True)
    except ValueError as error:
        raise SystemExit(f"{installer}:{line_number}: cannot parse command: {error}")
    if not words:
        raise SystemExit(f"{installer}:{line_number}: missing connector command")
    command, *arguments = words
    option_names = [argument[2:] for argument in arguments if argument.startswith("--")]
    if not option_names:
        print(f"checked {line_number}: connector {command} (no flags)")
        continue

    if command not in flag_cache:
        help_result = subprocess.run(
            [connector, command, "-h"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        help_text = help_result.stdout
        flag_cache[command] = set(
            re.findall(r"(?m)^\s+-([A-Za-z][A-Za-z0-9-]*)\s", help_text)
        )
    supported = flag_cache[command]
    unsupported = sorted(set(option_names) - supported)
    if unsupported:
        raise SystemExit(
            f"{installer}:{line_number}: connector {command} uses unsupported "
            f"flags {unsupported}; supported flags are {sorted(supported)}"
        )
    print(f"checked {line_number}: connector {command} flags {', '.join(option_names)}")

print(f"validated {len(invocations)} connector.exe invocations")
PY
