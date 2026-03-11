package launcher

import (
	"context"
	"log/slog"
	"time"
)

var (
	defaultBackoff           = []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second}
	defaultCrashThreshold    = 3
	defaultProbationDuration = 10 * time.Minute
	defaultKillTimeout       = 30 * time.Second
	defaultCrashWindow       = 5 * time.Minute
)

// Config configures the launcher behavior.
type Config struct {
	// Required fields.
	AppName         string // display name
	ChildBinaryName string // binary filename inside versions/current/
	DataDir         string // state, versions, IPC files
	InstallDir      string // where the launcher binary should live
	EnvVarName      string // env var set on child process (e.g. "MYAPP_LAUNCHER_STATE_DIR")

	// Optional with sensible defaults.
	ChildArgs         []string        // args forwarded to child (default: none)
	Backoff           []time.Duration // restart delays (default: [2s, 5s, 15s])
	CrashThreshold    int             // crashes before rollback (default: 3)
	CrashWindow       time.Duration   // crash count resets after this duration (default: 5min)
	ProbationDuration time.Duration   // new version probation (default: 10min)
	KillTimeout       time.Duration   // SIGTERM → SIGKILL escalation (default: 30s)

	// Pluggable components.
	UI        UI        // nil = headless
	Fetcher   Fetcher   // nil = no bootstrap/updates
	Registrar Registrar // nil = skip OS registration
}

// Launcher supervises a child process with versioned deployments and rollback.
type Launcher struct {
	cfg   Config
	state *State
	lock  *lockfile
}

// New creates a Launcher with the given config, applying defaults for unset fields.
func New(cfg Config) *Launcher {
	if len(cfg.Backoff) == 0 {
		cfg.Backoff = defaultBackoff
	}
	if cfg.CrashThreshold == 0 {
		cfg.CrashThreshold = defaultCrashThreshold
	}
	if cfg.CrashWindow == 0 {
		cfg.CrashWindow = defaultCrashWindow
	}
	if cfg.ProbationDuration == 0 {
		cfg.ProbationDuration = defaultProbationDuration
	}
	if cfg.KillTimeout == 0 {
		cfg.KillTimeout = defaultKillTimeout
	}
	return &Launcher{cfg: cfg}
}

// Status returns the current launcher state (for --status flag).
func (l *Launcher) Status() *State {
	if l.state == nil {
		s, err := loadState(l.cfg.DataDir)
		if err != nil {
			return &State{}
		}
		return s
	}
	return l.state
}

// Run executes the main supervisor loop. Returns the process exit code.
func (l *Launcher) Run(ctx context.Context) int {
	// Self-relocate on first run
	if l.cfg.InstallDir != "" {
		relocated, err := selfRelocate(l.cfg.InstallDir)
		if err != nil {
			slog.Error("self-relocation failed (continuing from current location)", "error", err)
		}
		if relocated {
			return 0
		}
	}

	// Ensure data directory exists
	if err := ensureDataDir(l.cfg.DataDir); err != nil {
		slog.Error("failed to create data directory", "error", err)
		return 1
	}

	// Acquire singleton lock
	l.lock = newLockfile(l.cfg.DataDir)
	if err := l.lock.Acquire(); err != nil {
		slog.Error("failed to acquire lockfile", "error", err)
		return 1
	}
	defer l.lock.Release()

	// Load state
	var err error
	l.state, err = loadState(l.cfg.DataDir)
	if err != nil {
		slog.Error("failed to load state", "error", err)
		return 1
	}

	// Startup recovery
	deleteStateTmpFiles(l.cfg.DataDir)
	cleanStagingDir(l.cfg.DataDir)
	recoverInterruptedSwap(l.cfg.DataDir)

	// Ensure version directories exist
	if err := ensureVersionDirs(l.cfg.DataDir); err != nil {
		slog.Error("failed to create version directories", "error", err)
		return 1
	}

	// Bootstrap download if no current version
	if !hasCurrentVersion(l.cfg.DataDir) {
		if l.cfg.Fetcher == nil {
			slog.Error("no child binary and no fetcher configured")
			if l.cfg.UI != nil {
				l.cfg.UI.ShowError("No application found. Please reinstall.")
			}
			return 1
		}

		if l.cfg.UI != nil {
			l.cfg.UI.ShowSplash("Setting up " + l.cfg.AppName + "...")
		}

		if err := bootstrapDownload(ctx, &l.cfg); err != nil {
			slog.Error("bootstrap download failed", "error", err)
			if l.cfg.UI != nil {
				l.cfg.UI.ShowError("Setup failed: " + err.Error())
			}
			return 1
		}
	}

	return l.supervisorLoop(ctx)
}

func (l *Launcher) supervisorLoop(ctx context.Context) int {
	crashIndex := 0 // index into backoff array

	for {
		// Check crash threshold
		if l.state.CrashCount >= l.cfg.CrashThreshold {
			if hasPreviousVersion(l.cfg.DataDir) && l.state.RolledBackFrom != l.state.PreviousVersion {
				slog.Warn("crash threshold reached, rolling back",
					"crash_count", l.state.CrashCount,
					"from", l.state.CurrentVersion,
					"to", l.state.PreviousVersion)

				if l.cfg.UI != nil {
					l.cfg.UI.ShowSplash("Recovering...")
				}

				if err := rollback(l.cfg.DataDir); err != nil {
					slog.Error("rollback failed", "error", err)
					return 1
				}

				// Swap state
				l.state.RolledBackFrom = l.state.CurrentVersion
				l.state.CurrentVersion, l.state.PreviousVersion = l.state.PreviousVersion, l.state.CurrentVersion
				l.state.CrashCount = 0
				l.state.CrashWindowStart = time.Time{}
				l.state.ProbationUntil = time.Time{}
				// Keep RolledBackFrom set (anti-oscillation guard)
				crashIndex = 0

				if err := saveState(l.cfg.DataDir, l.state); err != nil {
					slog.Error("failed to save state after rollback", "error", err)
				}
				continue
			}

			slog.Error("crash loop with no viable rollback target",
				"crash_count", l.state.CrashCount)
			if l.cfg.UI != nil {
				l.cfg.UI.ShowError(l.cfg.AppName + " is unable to start. Please reinstall or contact support.")
			}
			return 1
		}

		// Delete old heartbeat before spawning
		deleteHeartbeat(l.cfg.DataDir)

		if l.cfg.UI != nil {
			l.cfg.UI.ShowSplash("Starting " + l.cfg.AppName + "...")
		}

		binaryPath := childBinaryPath(l.cfg.DataDir, l.cfg.ChildBinaryName)
		child, err := spawnChild(binaryPath, l.cfg.ChildArgs, l.cfg.EnvVarName, l.cfg.DataDir)
		if err != nil {
			slog.Error("failed to spawn child", "path", binaryPath, "error", err)
			l.recordCrash()
			crashIndex = l.sleepBackoff(ctx, crashIndex)
			continue
		}

		slog.Info("child spawned", "pid", child.cmd.Process.Pid, "path", binaryPath)

		// Wait for child to exit, monitoring heartbeat
		exitCode := l.waitForChild(ctx, child)

		// Probation check: if child ran long enough and heartbeat was touched, mark stable
		if !l.state.ProbationUntil.IsZero() &&
			heartbeatTouchedAfter(l.cfg.DataDir, child.spawnTime) &&
			child.WallRuntime() > l.cfg.ProbationDuration {

			slog.Info("version survived probation, marking stable",
				"version", l.state.CurrentVersion,
				"runtime", child.WallRuntime())

			l.state.resetCrashState()
			if err := saveState(l.cfg.DataDir, l.state); err != nil {
				slog.Error("failed to save state after probation clear", "error", err)
			}
		}

		// Handle exit
		if shutdownRequested(l.cfg.DataDir) {
			// Check for pending update
			if pending := readPendingUpdate(l.cfg.DataDir); pending != nil {
				slog.Info("child requested update", "version", pending.Version)

				if l.cfg.UI != nil {
					l.cfg.UI.ShowSplash("Updating to " + pending.Version + "...")
				}

				if err := performUpdate(ctx, &l.cfg, pending); err != nil {
					slog.Error("update failed, restarting current version", "error", err)
					cleanStagingDir(l.cfg.DataDir)
				} else {
					// Update succeeded
					l.state.PreviousVersion = l.state.CurrentVersion
					l.state.CurrentVersion = pending.Version
					l.state.resetCrashState()
					l.state.ProbationUntil = time.Now().Add(l.cfg.ProbationDuration)
					if err := saveState(l.cfg.DataDir, l.state); err != nil {
						slog.Error("failed to save state after update", "error", err)
					}
				}

				deletePendingUpdate(l.cfg.DataDir)
				deleteShutdownFile(l.cfg.DataDir)
				crashIndex = 0
				continue
			}

			// Clean shutdown — no update
			slog.Info("child requested clean shutdown")
			deleteShutdownFile(l.cfg.DataDir)
			return 0
		}

		// No shutdown_requested — treat as crash regardless of exit code
		slog.Warn("child exited unexpectedly", "exit_code", exitCode,
			"runtime", child.WallRuntime())

		// Clean up stale pending update on crash
		deletePendingUpdate(l.cfg.DataDir)

		l.recordCrash()
		crashIndex = l.sleepBackoff(ctx, crashIndex)
	}
}

// waitForChild waits for the child to exit, monitoring the heartbeat file
// to dismiss the splash screen.
func (l *Launcher) waitForChild(ctx context.Context, cp *childProcess) int {
	heartbeatChecked := false
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cp.Done():
			return cp.ExitCode()
		case <-ticker.C:
			if !heartbeatChecked && heartbeatTouchedAfter(l.cfg.DataDir, cp.spawnTime) {
				heartbeatChecked = true
				ticker.Stop()
				slog.Info("child is healthy (heartbeat detected)")
				l.state.LastHealthyAt = time.Now()
				saveState(l.cfg.DataDir, l.state)
				if l.cfg.UI != nil {
					l.cfg.UI.HideSplash()
				}
			}
		case <-ctx.Done():
			slog.Info("context cancelled, terminating child")
			terminateWithEscalation(cp, l.cfg.KillTimeout)
			return cp.ExitCode()
		}
	}
}

func (l *Launcher) recordCrash() {
	now := time.Now()

	// Reset crash count if the window has expired
	if !l.state.CrashWindowStart.IsZero() && now.Sub(l.state.CrashWindowStart) > l.cfg.CrashWindow {
		l.state.CrashCount = 0
		l.state.CrashWindowStart = time.Time{}
	}

	l.state.CrashCount++
	if l.state.CrashWindowStart.IsZero() {
		l.state.CrashWindowStart = now
	}
	if err := saveState(l.cfg.DataDir, l.state); err != nil {
		slog.Error("failed to save state after crash", "error", err)
	}
}

func (l *Launcher) sleepBackoff(ctx context.Context, index int) int {
	delay := l.cfg.Backoff[index]
	slog.Info("waiting before restart", "delay", delay, "crash_count", l.state.CrashCount)

	select {
	case <-time.After(delay):
	case <-ctx.Done():
	}

	// Advance index, cap at last element
	if index < len(l.cfg.Backoff)-1 {
		return index + 1
	}
	return index
}

// Executable returns the path to the child binary in versions/current/.
func (l *Launcher) Executable() string {
	return childBinaryPath(l.cfg.DataDir, l.cfg.ChildBinaryName)
}

// DataDir returns the configured data directory.
func (l *Launcher) DataDir() string {
	return l.cfg.DataDir
}

// Shutdown writes shutdown_requested and can be called from the child
// to coordinate a clean exit. This is primarily for testing; in production,
// the child uses the child/ package.
func Shutdown(dataDir string) error {
	return writeShutdownRequested(dataDir)
}

// RequestUpdate writes pending_update.json and shutdown_requested.
// This is primarily for testing; in production, the child uses the child/ package.
func RequestUpdate(dataDir string, r *Release) error {
	if err := writePendingUpdate(dataDir, r); err != nil {
		return err
	}
	return writeShutdownRequested(dataDir)
}
