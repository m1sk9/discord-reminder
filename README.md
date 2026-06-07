> [!CAUTION]
>
> We have migrated to [chime](https://github.com/m1sk9/chime), which was reimplemented in Rust to improve safety and robustness.
> 
> Although this project started as a personal one, we are now expanding the scope of maintenance and continuing development so that others can use [chime](https://github.com/m1sk9/chime) as well.
> 
> discord-reminder will be discontinued as of June 7, 2026, at 8:00 PM. Since it is not open-source software, it will not enter maintenance mode. This is a complete discontinuation. Use at your own risk.

# discord-reminder

[![CI](https://github.com/m1sk9/discord-reminder/actions/workflows/ci.yml/badge.svg)](https://github.com/m1sk9/discord-reminder/actions/workflows/ci.yml)
[![Release](https://github.com/m1sk9/discord-reminder/actions/workflows/release.yml/badge.svg)](https://github.com/m1sk9/discord-reminder/actions/workflows/release.yml)

A lightweight Discord webhook reminder built with Go.

## Features

- Per-reminder weekday and `HH:MM` schedule, validated at startup (fail-fast)
- Minute-granularity matching with deduplication so a reminder fires at most once per minute regardless of tick frequency
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Secrets isolated from configuration (`secrets.toml` is read-only to the service user, `0640`)
- No runtime dependencies beyond the Go standard library and `github.com/BurntSushi/toml`

## Installation

### One-line install (latest release)

```sh
curl -fsSL https://raw.githubusercontent.com/m1sk9/discord-reminder/main/install.sh | sudo sh
```

The script auto-detects architecture (`amd64` / `arm64`), downloads the matching release tarball,
verifies its SHA-256, and installs:

| Path | Purpose |
| --- | --- |
| `/usr/local/bin/discord-reminder` | Binary (mode `0755`, owner `root:root`) |
| `/etc/discord-reminder/config.toml` | Configuration (mode `0644`, owner `root:root`) |
| `/etc/discord-reminder/secrets.toml` | Webhook URLs (mode `0640`, owner `root:discord-reminder`) |
| `/etc/systemd/system/discord-reminder.service` | systemd unit |
| system user `discord-reminder` | Unprivileged runtime user |

Existing `config.toml` / `secrets.toml` are never overwritten.

### Pinning a specific version

```sh
curl -fsSL https://raw.githubusercontent.com/m1sk9/discord-reminder/main/install.sh \
  | sudo VERSION=v1.0.0 sh
```

### Offline / manual install

Download the release tarball and run the bundled `install.sh`:

```sh
tar xzf discord-reminder-v1.0.0-linux-amd64.tar.gz
cd discord-reminder-v1.0.0-linux-amd64
sudo ./install.sh
```

The script detects that the artifacts already sit next to it and skips the download step.

### Supported environments

- Linux with `systemd`
- `glibc` and `musl` distributions: shadow-utils `useradd` (Debian, Ubuntu, RHEL, Fedora, openSUSE, …) and BusyBox `adduser` (Alpine) are both detected automatically
- Architectures: `amd64`, `arm64`

### Environment overrides

The installer accepts these environment variables:

| Variable | Default | Notes |
| --- | --- | --- |
| `REPO` | `m1sk9/discord-reminder` | GitHub `owner/repo` to download from |
| `VERSION` | `latest` | Release tag (e.g. `v1.0.0`) or `latest` |
| `PREFIX` | `/usr/local/bin` | Binary install directory |
| `CONFDIR` | `/etc/discord-reminder` | Configuration directory |
| `USER_NAME` | `discord-reminder` | Service user name |

## Configuration

### `config.toml`

```toml
[system]
log_level = "info"             # debug | info | warn | error
tick_interval_sec = 30         # 1..60; how often the scheduler checks for matches
timezone = "Asia/Tokyo"        # any IANA name; matching uses this zone

[[reminders]]
name = "daily-standup"          # must be unique within the file
time = "09:30"                  # HH:MM, 24h, in the configured timezone
days = ["mon", "tue", "wed", "thu", "fri"]
message = "Daily standup starting in 5 minutes."
webhook = "team"                # key into [webhooks] in secrets.toml
```

`days` accepts `sun`, `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, or the special value `every` (expanded to all seven weekdays).

### `secrets.toml`

```toml
[webhooks]
team = "https://discord.com/api/webhooks/123456789/abc..."
alerts = "https://discord.com/api/webhooks/987654321/xyz..."
```

Each `webhook` referenced by a reminder must have a matching key here, otherwise startup fails.

### Validation

All validation runs at load time and aborts with a descriptive error before the scheduler starts:

- `tick_interval_sec` outside `1..60`
- Unknown timezone
- Missing or duplicated reminder `name`
- `time` not in `HH:MM` form, hour `>23`, or minute `>59`
- Empty `days`, unknown weekday
- Webhook key not present in `secrets.toml`
- Webhook URL not a valid request URI
- Empty `message`
- Zero reminders defined

## Security notes

The systemd unit applies the following sandboxing:

- `User=discord-reminder` (unprivileged)
- `NoNewPrivileges=true`
- `ProtectSystem=strict`, `ProtectHome=true`
- `PrivateTmp=true`, `PrivateDevices=true`
- `ProtectKernelTunables/Modules/Logs=true`, `ProtectControlGroups=true`
- `ProtectClock=true`, `ProtectHostname=true`
- `RestrictNamespaces`, `RestrictRealtime`, `RestrictSUIDSGID`
- `LockPersonality`, `MemoryDenyWriteExecute`
- `CapabilityBoundingSet=` and `AmbientCapabilities=` (all capabilities dropped)
- `SystemCallFilter=@system-service ~@privileged @resources`

`secrets.toml` is created with mode `0640` and group ownership `discord-reminder`, so only `root` and the service user can read it.

## License

See `LICENSE`.
