package launcher

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// selfRelocate copies the running binary to the install directory and relaunches
// from there. Returns true if relocation happened (caller should exit).
// Returns false if already running from the install directory.
func selfRelocate(cfg *Config) (bool, error) {
	installDir := cfg.InstallDir
	execPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return false, fmt.Errorf("eval symlinks: %w", err)
	}

	// On macOS, check if we're inside a .app bundle and compare the bundle root
	if runtime.GOOS == "darwin" {
		if idx := strings.Index(execPath, ".app/"); idx != -1 {
			bundlePath := execPath[:idx+4] // up to and including ".app"
			bundleDir := filepath.Dir(bundlePath)
			if pathsEqual(bundleDir, installDir) {
				return false, nil // already in install dir
			}
		} else if pathsEqual(filepath.Dir(execPath), installDir) {
			return false, nil
		}
	} else if pathsEqual(filepath.Dir(execPath), installDir) {
		// Already inside InstallDir; if the caller requested a specific
		// filename and we're not it, that's a name drift — but renaming a
		// running binary is OS-specific, so treat as already-relocated and
		// continue. The caller can rename out-of-band on next launch.
		return false, nil
	}

	slog.Info("first run detected, relocating to install directory",
		"from", execPath, "to", installDir)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return false, fmt.Errorf("create install dir: %w", err)
	}

	destPath := filepath.Join(installDir, relocateDestName(cfg, execPath))

	// On macOS with .app bundle, move the entire bundle. LauncherBinaryName
	// doesn't apply here — the bundle name is determined by Info.plist.
	if runtime.GOOS == "darwin" {
		if idx := strings.Index(execPath, ".app/"); idx != -1 {
			bundlePath := execPath[:idx+4]
			destPath = filepath.Join(installDir, filepath.Base(bundlePath))
			if err := os.Rename(bundlePath, destPath); err != nil {
				// Rename failed (cross-device), fall back to copy
				if err := copyDir(bundlePath, destPath); err != nil {
					return false, fmt.Errorf("copy app bundle: %w", err)
				}
			}
			removeQuarantine(destPath)

			// Relaunch from the new bundle
			newExec := filepath.Join(destPath, execPath[idx+4:])
			return true, relaunch(newExec, cfg)
		}
	}

	// Copy single binary
	if err := copyFile(execPath, destPath); err != nil {
		return false, fmt.Errorf("copy binary: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		return false, fmt.Errorf("chmod: %w", err)
	}

	return true, relaunch(destPath, cfg)
}

func relocateDestName(cfg *Config, execPath string) string {
	if cfg != nil && cfg.LauncherBinaryName != "" {
		return cfg.LauncherBinaryName
	}
	return filepath.Base(execPath)
}

func relaunch(binaryPath string, cfg *Config) error {
	args := os.Args[1:]
	if cfg != nil && cfg.RelaunchArgs != nil {
		args = cfg.RelaunchArgs(args)
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Start()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func pathsEqual(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	// Case-insensitive on macOS and Windows (case-preserving filesystems)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
