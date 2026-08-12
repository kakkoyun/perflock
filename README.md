# perflock

Perflock serializes benchmarks on a shared host. A system daemon owns a FIFO
queue, and clients hold either an exclusive or shared lock while their command
runs.

Use an exclusive lock for benchmarks:

```sh
perflock command arg...
```

Use a shared lock for work that may disturb benchmarks but can run alongside
other shared work:

```sh
perflock -shared command arg...
```

`perflock -list` prints the current holder and waiting clients.

## Platform support

| Capability | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Shared and exclusive lock | Yes | Yes | Yes |
| Transport | Unix socket | Unix socket | Named pipe |
| Client identity in `-list` | Peer UID | Peer UID | Process token |
| CPU performance control | `cpufreq` frequency range | Not available | Processor state percentage |
| Other performance control | None | `pmset` power mode | None |
| System service | systemd or Upstart | launchd | Windows service |

Linux and Windows default to `-governor 90%`. The value is a percentage of the
available Linux frequency range and a percentage of the Windows maximum
processor state. If a default request cannot be applied, perflock warns and
continues. An explicit `-governor` request fails if the host cannot apply it.

macOS defaults to `-governor none` because macOS provides no supported CPU
frequency or core-affinity API. `-power-mode auto|low|high` changes the macOS
system power mode while an exclusive lock is held, then restores both charger
and battery settings. Not every Mac supports High Power Mode. Perflock verifies
the requested mode by reading it back and fails if the hardware ignores it.

On macOS, each run also reports battery use, Low Power Mode, recorded thermal or
performance warnings, and the performance/efficiency core split when those
values are available. These observations do not guarantee benchmark stability.

## Build and install

Build the binary from the repository root:

```sh
go build ./cmd/perflock
```

### Linux

```sh
sudo ./install.bash
```

The installer uses systemd or Upstart when available. Otherwise, it starts the
daemon directly and warns that it will not restart at boot.

### macOS

```sh
sudo ./install.bash
```

The installer copies the binary to `/usr/local/bin/perflock` and installs the
system launch daemon from `init/launchd`. The daemon runs as root so it can
create `/var/run/perflock.socket` and honor `pmset` requests.

### Windows

From an elevated PowerShell:

```powershell
go build -o perflock.exe ./cmd/perflock
.\init\windows\install.ps1
```

The installer registers an automatically started Windows service. See
`init/windows/README.md` for access controls and removal instructions.

## Manual daemon startup

On Linux or macOS:

```sh
sudo perflock -daemon
```

On Windows, an elevated interactive shell may run:

```powershell
perflock.exe -daemon
```

Use `-socket` to select a different local endpoint. Linux also accepts abstract
Unix socket names beginning with `@`.
