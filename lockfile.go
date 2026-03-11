package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const lockFileName = "launcher.lock"

type lockfile struct {
	path string
}

func newLockfile(dataDir string) *lockfile {
	return &lockfile{path: filepath.Join(dataDir, lockFileName)}
}

// Acquire attempts to take the lockfile. Returns an error if another
// launcher instance is already running.
func (l *lockfile) Acquire() error {
	// Check for existing lock
	if data, err := os.ReadFile(l.path); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if processAlive(pid) {
				return fmt.Errorf("already running (pid %d)", pid)
			}
		}
		// Stale lockfile — previous launcher crashed
	}

	pid := os.Getpid()
	if err := os.WriteFile(l.path, []byte(strconv.Itoa(pid)), 0600); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}
	return nil
}

// Release removes the lockfile.
func (l *lockfile) Release() {
	os.Remove(l.path)
}
