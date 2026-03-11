package launcher

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDataDir returns the platform-appropriate data directory for the given app name.
//
//   - macOS:   ~/Library/Application Support/{appName}/
//   - Windows: %LOCALAPPDATA%/{appName}/
//   - Linux:   ~/.local/share/{appName}/
func DefaultDataDir(appName string) string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", appName)
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), appName)
	default: // linux and others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", appName)
	}
}

// DefaultInstallDir returns the platform-appropriate install directory for the launcher binary.
//
//   - macOS:   /Applications/
//   - Windows: %LOCALAPPDATA%/{appName}/
//   - Linux:   ~/.local/bin/
func DefaultInstallDir(appName string) string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications"
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), appName)
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "bin")
	}
}

// ensureDataDir creates the data directory with 0700 permissions.
func ensureDataDir(dataDir string) error {
	return os.MkdirAll(dataDir, 0700)
}
