# HRMS Connector

HRMS Connector is a Go background application for factory LANs. It receives ZKTeco ADMS requests at `/iclock/*`, forwards them to the configured HRMS endpoint over HTTPS, and provides a local browser interface for configuration and device setup. Failed upstream requests return `503` so devices can retry.

The application targets Go 1.22 or later. Its UI is embedded vanilla HTML/CSS/JavaScript and requires no frontend build.

## Build and test

```sh
make test
make build-linux
make windows
make installer
```

Windows builds use `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`; `make windows` produces `dist/connector.exe` and `make installer` produces `dist/HRMS-Connector-Setup-<version>.exe`. Set `VERSION=1.2.3` to version the outputs. `makensis` is required for the installer target.

Unsigned Windows builds can cause Microsoft SmartScreen to show **Unknown publisher**. Set `SIGN_CERT` and `SIGN_PASSWORD` to sign the executables and installer with `osslsigncode`.

## Windows installation

Run the setup executable as Administrator. Verify the HRMS Connection Key in the installer, select the relay and local UI ports, and complete the optional network page. The installer checks that selected ports can bind and are not in Windows' excluded TCP port ranges. New installs use relay port `17081` and local UI port `17082`; both must be between `1024` and `49151`. An upgrade keeps the ports already saved in `config.json` unless new values are selected.

The installer creates and starts the `HRMSConnector` Windows service, waits up to 30 seconds for the UI to respond, adds the relay firewall rule, and creates Start Menu and (by default) Desktop shortcuts. The UI listens only on `127.0.0.1`; open it on the connector PC at `http://127.0.0.1:<ui-port>/`. The Finish page can open Device Setup directly.

Silent installation supports:

```text
HRMS-Connector-Setup-<version>.exe /S /KEY=<connection-key> /PORT=17081 /UIPORT=17082 /NETFIX=1 /DESKTOP=1
```

For an upgrade, omit `/KEY`, `/PORT`, and `/UIPORT` to retain the saved key and ports. `/NETFIX=0` skips the optional network step and `/DESKTOP=0` omits the Desktop shortcut. The key must be verified with the HRMS server during installation.

To uninstall, use **Apps & features** or run `uninstall.exe` from the installation directory as Administrator. This stops/removes the service and removes the firewall rule and shortcuts. If this installation changed the PC from DHCP to static addressing, the uninstaller offers to restore DHCP.

## First run and device setup

On first run, the browser opens the Device Setup page. It shows the HRMS company/site and the values to enter on the time clock:

- Server Mode: **ADMS**
- Server Address: this PC's LAN address
- Port: the configured relay port
- Enable Domain Name, HTTPS, and Proxy: **OFF**

The page also shows best-effort suggestions for device IP, subnet mask, gateway, and DNS. Verify suggested addresses with the network administrator before using them. Follow `Menu > Comm. > Cloud Server Setting`; set the correct device date/time and set Punch State to Auto. The live indicator updates when a device contacts the connector. The optional **Set a PIN** action protects later settings changes.

Settings can change the relay port, but the UI port is fixed after installation. Changing the relay port requires updating the port on every device. If the configured relay port is occupied, the connector logs the bind failure, displays a UI warning, and retries that exact port; it does not silently choose another port.

## Running manually

On Windows, run `connector.exe run` from an elevated terminal to run in the foreground, or use `connector.exe install`, `start`, `stop`, and `uninstall` to control the Windows service. Running `connector.exe` with no arguments opens the local UI in the default browser.

On Linux, run:

```sh
go run ./cmd/connector run --data-dir ./data
```

The Linux default data directory is `/etc/hrms-connector`; Windows uses `%ProgramData%\HRMS Connector`. The first start creates `config.json` with relay port `17081` and UI port `17082`. Existing configuration files keep their saved ports. The UI is always bound to `127.0.0.1`.

Useful commands:

```text
connector run [--data-dir PATH]
connector verify-key <key>
connector wait-ready [--timeout 30s] [--data-dir PATH]
connector check-ports [--relay PORT] [--ui PORT]
connector netfix [--status|--apply|--restore]
connector version
```

## Network address notes

The PC IP card reports whether the selected adapter uses fixed or automatic addressing. On Windows, the connector compares default-route and interface metrics, can use observed device subnets to resolve ties, and asks the user to choose when the route remains ambiguous. A remembered adapter selection is stored in the connector data directory. Network suggestions for the time clock are best effort and do not modify the PC address.

The optional **Fix IP address** action only changes DHCP addressing to static while preserving the current IP, prefix, gateway, and DNS. The router must not assign that same address to another device; a DHCP reservation is recommended. If configuration verification fails, the connector attempts to restore DHCP.

## Connection key and diagnostics

Connection keys use the format `HRMS1.<base64url(JSON)>`, where the JSON contains version, HTTPS server URL, and token. The UI masks the token. The relay logs request metadata only, not request bodies or credentials; logs rotate at 5 MB with three backups. Data and log files live in the configured data directory.

## Real Windows acceptance checklist

These checks require a Windows machine or VM and are not covered by the Linux cross-build:

- Install on a clean VM, verify the key and selected ports, then confirm the service starts.
- Reboot without logging in and confirm the service remains running and the local UI opens from its shortcut.
- Connect a test device; confirm live serial/last-seen status and that attendance requests reach HRMS.
- Confirm the PC IP value is unchanged by installation or the optional DHCP-to-static action; test adapter selection where default-route metrics tie.
- Upgrade in place and confirm saved settings and shortcuts remain; uninstall and confirm the service, firewall rule, shortcuts, and optional DHCP restoration are handled.
