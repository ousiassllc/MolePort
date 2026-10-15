# MolePort

[日本語](README_ja.md)

SSH port forwarding manager with a daemon+client architecture.

Reads host information from `~/.ssh/config` and lets you configure, connect, and disconnect port forwarding via CLI or TUI.

```mermaid
graph LR
    A["CLI / TUI<br/>(Client)"] <-->|"Unix Socket<br/>(JSON-RPC 2.0)"| B["Daemon<br/><br/>SSHManager<br/>FwdManager<br/>ConfigMgr"]
```

## Features

- **SSH config integration** --- Automatically reads hosts from `~/.ssh/config` (supports Include directives)
- **3 forwarding types** --- Local (-L) / Remote (-R) / Dynamic SOCKS5 (-D)
- **Real-time monitoring** --- Displays connection status, uptime, and transferred data volume
- **Auto-reconnect** --- Automatic retry with exponential backoff
- **Session restore** --- Automatically restores previous active forwarding on startup
- **Daemon+client** --- Background daemon manages SSH connections; CLI/TUI operates as a client
- **Desktop GUI** --- `moleport-gui` (Linux / macOS / Windows) with system tray residency and an embedded daemon

## Requirements

- Go 1.25+
- Linux / macOS (CLI / TUI)
- Linux / macOS / Windows (GUI, see [Desktop GUI](#desktop-gui))

## Installation

```bash
git clone https://github.com/ousiassllc/MolePort.git
cd MolePort
make install
```

`make install` runs `go install` to place the binary in `$(go env GOPATH)/bin`.
If that directory is not in your PATH, add it to your shell configuration:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

To build only (outputs to `./bin/moleport`):

```bash
make build
```

## Desktop GUI

`moleport-gui` is a desktop app that embeds the daemon and shows the dashboard in a native window. It keeps running in the system tray when the window is closed. It is distributed as separate assets on the [GitHub Releases](https://github.com/ousiassllc/MolePort/releases) page (the `moleport` CLI/TUI assets are unchanged).

| Asset | Platform |
|-------|----------|
| `moleport-gui_<version>_linux_amd64.tar.gz` / `moleport-gui_<version>_linux_arm64.tar.gz` | Linux x64 / arm64 |
| `moleport-gui_<version>_darwin_universal.zip` | macOS 11+ (Apple Silicon / Intel) |
| `moleport-gui_<version>_windows_amd64.zip` | Windows 10 (1803+) / 11, x64 |
| `checksums_gui.txt` | SHA-256 checksums of the assets above |

Verify a downloaded asset with `sha256sum -c --ignore-missing checksums_gui.txt` (Linux) or `shasum -a 256 -c --ignore-missing checksums_gui.txt` (macOS). Then extract the archive and run `moleport-gui` (Linux), `MolePort.app` (macOS) or `moleport-gui.exe` (Windows).

### Linux requirements

- GTK 3 and WebKitGTK 4.1 (`libgtk-3-0` and `libwebkit2gtk-4.1-0`; e.g. `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0`)
- **System tray**: a StatusNotifierItem (D-Bus) host is required. KDE Plasma, XFCE and similar desktops support it out of the box; GNOME needs the [AppIndicator / KStatusNotifierItem extension](https://extensions.gnome.org/extension/615/appindicator-support/). Without a tray host, closing the window asks for quit confirmation instead of hiding to the tray.

### Unsigned builds (warnings on first launch)

The initial releases are not code-signed or notarized, so your OS may show a warning.

- **macOS** --- "MolePort.app cannot be opened because the developer cannot be verified": right-click (or Control-click) `MolePort.app` in Finder, choose **Open**, then click **Open** in the dialog. This is needed only the first time. If the app is still blocked, run `xattr -dr com.apple.quarantine MolePort.app`.
- **Windows** --- SmartScreen shows "Windows protected your PC": click **More info**, then **Run anyway**. Windows 10 installs the WebView2 runtime on first launch if it is missing.

### Building the GUI from source

```bash
make gui-build                           # Requires Wails CLI v2 and the native WebView environment
make gui-build GUI_TAGS=gui,webkit2_41   # Linux with libwebkit2gtk-4.1-dev
```

## Quick Start

```bash
# 1. Start the daemon
moleport daemon start

# 2. Connect to an SSH host
moleport connect prod-server

# 3. Add a forwarding rule
moleport add --host prod-server --type local --local-port 8080 --remote-host localhost --remote-port 80

# 4. Start forwarding
moleport start web

# 5. Monitor with the TUI dashboard
moleport tui
```

## CLI Commands

Running `moleport` without a subcommand launches the TUI dashboard (equivalent to `moleport tui`).

| Command | Description |
|---------|-------------|
| `moleport daemon start` | Start the daemon in the background |
| `moleport daemon stop [--purge]` | Stop the daemon (`--purge`: clear state) |
| `moleport daemon status` | Show daemon status |
| `moleport daemon kill` | Force terminate an unresponsive daemon |
| `moleport connect <host>` | Connect to an SSH host |
| `moleport disconnect <host>` | Disconnect from an SSH host |
| `moleport add [flags]` | Add a forwarding rule |
| `moleport delete <name>` | Delete a forwarding rule |
| `moleport start <name>` | Start forwarding |
| `moleport stop <name> / --all` | Stop forwarding (`--all`: stop all) |
| `moleport list [--json]` | List hosts and forwarding rules |
| `moleport status [name]` | Show connection status summary |
| `moleport config [--json]` | Show configuration |
| `moleport reload` | Reload SSH config |
| `moleport tui` | Launch the TUI dashboard |
| `moleport update [--check]` | Auto-update to latest version (`--check`: check only) |
| `moleport version` | Show version information |
| `moleport help` | Show help |

## TUI Key Bindings

| Key | Action |
|-----|--------|
| `↑`/`k` `↓`/`j` | Select item |
| `Enter` | Toggle connect/disconnect |
| `Tab` | Switch pane |
| `d` | Disconnect selected forwarding |
| `x` | Delete selected forwarding |
| `t` | Change theme |
| `l` | Change language |
| `v` | Show version info |
| `/` | Focus command input |
| `?` | Show help |
| `Esc` | Cancel |
| `q` / `Ctrl+C` | Quit |

## Architecture

```mermaid
graph TD
    Client["CLI / TUI (Client)"]
    Client <-->|"Unix Socket (JSON-RPC 2.0)"| Daemon
    subgraph Daemon
        IPC["IPC Server<br/>(EventBroker + Handler)"]
        Core["Core Layer<br/>SSHManager / ForwardManager / ConfigManager"]
        Infra["Infrastructure Layer<br/>SSHConnection / SSHConfigParser / YAMLStore"]
        IPC --> Core --> Infra
    end
```

## Configuration

Config file: `~/.config/moleport/config.yaml`

```yaml
ssh_config_path: "~/.ssh/config"

reconnect:
  enabled: true
  max_retries: 10
  initial_delay: "1s"
  max_delay: "60s"
  keepalive_interval: "30s"

session:
  auto_restore: true

log:
  level: "info"
  file: "~/.config/moleport/moleport.log"

language: "ja"

tui:
  theme:
    base: "dark"           # "dark" | "light"
    accent: "violet"       # "violet" | "blue" | "green" | "cyan" | "orange"

update_check:
  enabled: true            # false to disable update checks
  interval: "24h"          # check interval
```

## Host Key Verification

MolePort verifies host keys using `~/.ssh/known_hosts`. In environments where host keys may change (e.g., Tailscale SSH), connections can fail with `knownhosts: key mismatch`.

Setting `StrictHostKeyChecking no` in your SSH config tells MolePort to skip host key verification for that host.

```
# ~/.ssh/config
Host tailscale-host
    HostName 100.64.x.x
    StrictHostKeyChecking no
```

You can also configure multiple hosts at once:

```
Host ts-host1 ts-host2 ts-host3
    StrictHostKeyChecking no
```

> **Warning**: `StrictHostKeyChecking no` completely disables host key verification. Use it only for hosts on trusted networks.

## Development

```bash
make help        # Show available targets
make build       # Build
make run         # Build and run
make test        # Run tests
make test-race   # Run tests with race detector
make vet         # Run go vet
make fmt         # Run go fmt
make lint        # Run golangci-lint
make linterly    # Run linterly (file line count check)
make gui-assets  # Generate GUI assets (templ generate + npm run build)
make gui-build   # Build the GUI (-tags gui; requires Wails CLI and a native WebView environment)
make setup-tools # Install development tools (linterly, golangci-lint)
make clean       # Remove build artifacts
```

### Git Hooks (lefthook)

[lefthook](https://github.com/evilmartians/lefthook) runs automated checks on commit and push.

| Hook | Checks |
|------|--------|
| pre-commit | `gofmt` formatting, `go vet`, `golangci-lint`, `linterly` |
| pre-push | `go test -race` (tests with race detector), `go build` (build check), `golangci-lint` |

Setup:

```bash
# Install lefthook (if not already installed)
go install github.com/evilmartians/lefthook@latest

# Enable Git hooks
lefthook install
```

## License

MIT
