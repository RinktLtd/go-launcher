# go-launcher

Crash-safe auto-updates for Go applications -- versioned deployments with automatic rollback when the new version fails.

## The Problem

Desktop and server applications need reliable update mechanisms. The common approaches in Go all have the same flaw: **self-surgery** -- the running binary replaces itself on disk, then restarts. If the new version crashes at startup, recovery logic never executes. If the replacement is interrupted (power loss, OOM kill), the installation is left in a broken state with no rollback path.

This is how Discord, Slack, and VS Code solved it years ago with [Squirrel.Windows](https://github.com/Squirrel/Squirrel.Windows): a thin launcher manages versioned directories side-by-side, and the switch to a new version is just launching from a different directory next time. The old version stays intact until the new one proves stable.

We could not find a Go library that implements this pattern. **go-launcher** does.

go-launcher is designed for Go applications that run persistently -- desktop apps, system services, or long-running agents. It is not a replacement for package managers or container orchestration; it targets the deployment gap where your binary runs directly on user machines and must update itself reliably.

## What It Does

go-launcher is a library you embed in a small launcher binary that supervises your actual application:

```
your-launcher          (thin binary, ~40 lines of your code + go-launcher)
  └── your-app         (your actual application, spawned as a child process)
```

The launcher handles:

- **Process supervision** -- spawn, monitor, restart with configurable backoff
- **Versioned directories** -- `versions/current/` and `versions/previous/` side-by-side
- **Crash detection + automatic rollback** -- if the new version crash-loops, swap back to the previous version
- **Update orchestration** -- download to staging, verify checksum, atomic rotation
- **Probation period** -- new versions must survive a configurable window before being marked stable
- **Anti-oscillation** -- prevents infinite swapping between two broken versions
- **Self-relocation** -- launcher copies itself from Downloads to a permanent location on first run
- **Singleton enforcement** -- PID lockfile prevents duplicate instances
- **File-based IPC** -- no sockets, no pipes; communication via env vars and files
- **Bootstrap download** -- if no child binary exists, download the latest version on first launch

## How Existing Libraries Compare

| Library | Approach | Rollback | Supervisor | Versioned dirs | Windows |
|---|---|---|---|---|---|
| [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) | Self-surgery | Apply-time only | No | No | Yes |
| [minio/selfupdate](https://github.com/minio/selfupdate) | Self-surgery | No | No | No | Yes |
| [sanbornm/go-selfupdate](https://github.com/sanbornm/go-selfupdate) | Self-surgery | No | No | No | Yes |
| [rhysd/go-github-selfupdate](https://github.com/rhysd/go-github-selfupdate) | Self-surgery | Apply-time only | No | No | Yes |
| [jpillora/overseer](https://github.com/jpillora/overseer) | Master/child | No | Yes | No | No |
| [fynelabs/selfupdate](https://github.com/fynelabs/selfupdate) | Self-surgery | Apply-time only | No | No | Yes |
| **go-launcher** | **External supervisor** | **Crash-based** | **Yes** | **Yes** | **Yes** |

**Apply-time rollback** means the `.old` file is restored if the rename/copy fails during the swap. It does not help if the new version starts successfully but crashes 30 seconds later.

**Crash-based rollback** means the launcher detects that the new version is crash-looping and automatically reverts to the previous known-good version -- even if the new version ran briefly before crashing.

> This table compares deployment architecture. Some of these libraries have strengths in other dimensions -- multi-backend support (GitHub/GitLab/S3), code signing verification, GOOS/GOARCH detection -- see each library's documentation for full feature sets.

## Quick Start

> For a runnable end-to-end demo, see the [`_example/`](./_example/) directory.

### Launcher side (your thin launcher binary)

### Launcher side (your thin launcher binary)

```go
package main

import (
    "context"
    "os"

    "github.com/rinktltd/go-launcher"
    "github.com/rinktltd/go-launcher/fetch"
)

func main() {
    l := launcher.New(launcher.Config{
        AppName:         "My App",
        ChildBinaryName: "my-app",
        DataDir:         launcher.DefaultDataDir("MyApp"),
        InstallDir:      launcher.DefaultInstallDir("MyApp"),
        EnvVarName:      "MYAPP_LAUNCHER_STATE_DIR",
        Fetcher:         fetch.GitHubRelease("myorg", "myapp", fetch.AssetPattern("my-app-*")),
    })

    os.Exit(l.Run(context.Background()))
}
```

### Child side (your actual application)

```go
package main

import (
    "os"

    "github.com/rinktltd/go-launcher/child"
)

func init() {
    // Must match the launcher's EnvVarName
    child.SetEnvVar("MYAPP_LAUNCHER_STATE_DIR")
}

func main() {
    // ... application init ...

    // Signal healthy startup
    if child.IsManaged() {
        child.TouchHeartbeat()
    }

    // ... application runs ...

    // When an update is available:
    if child.IsManaged() {
        child.RequestUpdate("1.2.0", "https://example.com/my-app-1.2.0", "sha256:abc123...")
        os.Exit(0)
    }
}
```

## Architecture

### On-Disk Layout

```
$DATA_DIR/
  launcher.json                       # persistent state (7 flat JSON fields)
  launcher.lock                       # PID lockfile
  heartbeat                           # touched by child after healthy init
  pending_update.json                 # written by child when update is available
  shutdown_requested                  # flag file for clean exit
  versions/
    current/                          # active version (opaque directory)
    previous/                         # rollback target
    staging/                          # download in progress
    rollback-tmp/                     # transient during atomic swap
```

### Communication Protocol

No sockets, no named pipes. The launcher sets an environment variable (e.g. `MYAPP_LAUNCHER_STATE_DIR`) pointing to the data directory. The child writes files to signal state changes.

| Direction | Signal | Mechanism |
|---|---|---|
| Launcher -> Child | "You are managed" | Environment variable |
| Child -> Launcher | "I'm healthy" | Touch `heartbeat` file |
| Child -> Launcher | "Update available" | Write `pending_update.json` + `shutdown_requested`, exit 0 |
| Child -> Launcher | "Shut down cleanly" | Write `shutdown_requested`, exit 0 |

The launcher always restarts the child unless `shutdown_requested` exists with exit code 0. An unexpected exit 0 (without the file) is treated as a crash -- this avoids ambiguity from stray `os.Exit(0)` calls.

### Supervisor Loop

```
on startup:
  self-relocate if first run
  acquire lockfile
  recover from any interrupted operations
  if no child binary exists: bootstrap download via Fetcher

loop:
  if crash threshold reached:
    if rollback target exists: rollback, continue
    else: exit 1

  spawn child
  wait for exit:
    heartbeat touched     -> mark healthy
    shutdown_requested    -> if update pending: download, rotate, continue
                            else: exit 0
    no shutdown_requested -> record crash, backoff, continue

  on exit: check if probation should be cleared
```

### Update Flow

```
child detects new version (your app's logic)
  -> child calls child.RequestUpdate(version, url, checksum)
  -> child exits 0
  -> launcher downloads to staging/
  -> launcher verifies checksum
  -> launcher rotates: current/ -> previous/, staging/ -> current/
  -> launcher sets probation period
  -> launcher spawns new version
  -> if new version crash-loops -> automatic rollback to previous/
```

### Rollback

Three atomic renames with crash recovery:

```
current/      -> rollback-tmp/
previous/     -> current/
rollback-tmp/ -> previous/
```

If interrupted at any point, the launcher recovers to a consistent state on next startup by inspecting which directories exist.

## Interfaces

go-launcher is interface-driven. Provide your own implementations or use the built-in ones.

### UI (optional)

```go
type UI interface {
    ShowSplash(status string)
    UpdateProgress(percent float64, status string)
    HideSplash()
    ShowError(msg string)
}
```

Pass `nil` for headless operation. All UI calls are nil-safe.

### Fetcher (required for updates/bootstrap)

```go
type Fetcher interface {
    LatestVersion(ctx context.Context) (*Release, error)
    Download(ctx context.Context, release *Release, dst io.Writer, progress func(float64)) error
}
```

Built-in: `fetch.GitHubRelease()`, `fetch.HTTP()`.

### Registrar (optional)

```go
type Registrar interface {
    RegisterLoginItem(binaryPath string) error
    UnregisterLoginItem() error
    RegisterService(binaryPath string, args []string) error
    UnregisterService() error
}
```

Built-in: `registrar.Launchd()`, `registrar.WindowsRun()`, `registrar.XDGAutostart()`, `registrar.Systemd()`.

## Configuration

```go
launcher.Config{
    // Required
    AppName         string          // display name
    ChildBinaryName string          // binary filename in versions/current/
    DataDir         string          // state, versions, IPC files
    InstallDir      string          // where the launcher lives permanently
    EnvVarName      string          // env var set on child process

    // Optional (sensible defaults)
    ChildArgs         []string        // args forwarded to child (default: none)
    Backoff           []time.Duration // restart delays (default: [2s, 5s, 15s])
    CrashThreshold    int             // crashes before rollback (default: 3)
    CrashWindow       time.Duration   // crash count resets after this (default: 5min)
    ProbationDuration time.Duration   // new version probation (default: 10min)
    KillTimeout       time.Duration   // SIGTERM -> SIGKILL escalation (default: 30s)

    // Pluggable (all optional except Fetcher if you want updates)
    UI        UI          // nil = headless
    Fetcher   Fetcher     // nil = no bootstrap/updates
    Registrar Registrar   // nil = skip OS registration
}
```

`launcher.DefaultDataDir(appName)` and `launcher.DefaultInstallDir(appName)` return platform-appropriate paths:

| Platform | DataDir | InstallDir |
|---|---|---|
| macOS | `~/Library/Application Support/{appName}/` | `/Applications/` |
| Windows | `%LOCALAPPDATA%\{appName}\` | `%LOCALAPPDATA%\{appName}\` |
| Linux | `~/.local/share/{appName}/` | `~/.local/bin/` |

### Logging

go-launcher logs via `log/slog`. Configure `slog.SetDefault()` before calling `Run()` to control output level and format.

## Design Decisions

**File-based IPC over sockets/pipes.** Files are debuggable (`cat heartbeat`), survive process crashes, and work identically across platforms. No protocol versioning, no serialization library, no connection management.

**Probation checked at exit time, not via polling.** When the child exits, the launcher checks whether it ran longer than the probation period and whether the heartbeat was touched. No background goroutine, no ticker. Probation only matters when something goes wrong.

**Always restart by default.** The launcher restarts the child on any exit (including exit 0) unless the `shutdown_requested` file exists. This eliminates ambiguity from multiple `os.Exit(0)` call sites in the child.

**Anti-oscillation guard.** After rolling back from version B to A, the `rolled_back_from` field prevents rolling back again to B if A also crashes. The launcher exits with code 1 instead of looping forever.

**Atomic state writes.** All persistent state uses write-to-tmp + rename. No truncated reads from interrupted writes.

## Security

go-launcher downloads binaries from the internet and executes them. The built-in fetchers enforce HTTPS. Downloaded artifacts are verified against SHA-256 checksums provided in the `Release.Checksum` field.

Code signing verification is not currently built in. If your threat model requires it, implement a custom `Fetcher` that verifies signatures before writing to the `dst` writer.

## Limitations

- The child must be a single binary (or a directory of files, but the launcher manages the directory as an opaque unit).
- The launcher does not update itself. Updating the launcher binary requires a separate mechanism (e.g. a new download from your website).
- No delta/incremental updates -- each version is a full download.
- No download resumption -- interrupted downloads restart from the beginning.

## License

MIT

---

go-launcher is maintained by [Rinkt](https://rinkt.com), where we use it to ship reliable updates for our RPA platform.
