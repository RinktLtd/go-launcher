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
func selfRelocate(installDir, binaryName string, transformArgs func([]string) []string) (bool, error) {
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
				return false, nil
			}
		} else if pathsEqual(filepath.Dir(execPath), installDir) {
			warnOnNameDrift(binaryName, execPath)
			return false, nil
		}
	} else if pathsEqual(filepath.Dir(execPath), installDir) {
		warnOnNameDrift(binaryName, execPath)
		return false, nil
	}

	slog.Info("first run detected, relocating to install directory",
		"from", execPath, "to", installDir)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return false, fmt.Errorf("create install dir: %w", err)
	}

	destPath := filepath.Join(installDir, relocateDestName(binaryName, execPath))

	// On macOS .app bundles, the bundle name comes from Info.plist; binaryName does not apply.
	if runtime.GOOS == "darwin" {
		if idx := strings.Index(execPath, ".app/"); idx != -1 {
			bundlePath := execPath[:idx+4]
			destPath = filepath.Join(installDir, filepath.Base(bundlePath))
			if err := os.Rename(bundlePath, destPath); err != nil {
				if err := copyDir(bundlePath, destPath); err != nil {
					return false, fmt.Errorf("copy app bundle: %w", err)
				}
			}
			removeQuarantine(destPath)

			newExec := filepath.Join(destPath, execPath[idx+4:])
			return true, relaunch(newExec, transformArgs)
		}
	}

	if err := copyFile(execPath, destPath); err != nil {
		return false, fmt.Errorf("copy binary: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		return false, fmt.Errorf("chmod: %w", err)
	}

	return true, relaunch(destPath, transformArgs)
}

func relocateDestName(binaryName, execPath string) string {
	if binaryName != "" {
		return filepath.Base(binaryName)
	}
	return filepath.Base(execPath)
}

func warnOnNameDrift(binaryName, execPath string) {
	if binaryName == "" {
		return
	}
	have := filepath.Base(execPath)
	if have != filepath.Base(binaryName) {
		slog.Warn("launcher running under unexpected filename; rename will not take effect on a running binary",
			"have", have, "want", binaryName)
	}
}

func resolveRelaunchArgs(transform func([]string) []string, args []string) []string {
	if transform == nil {
		return args
	}
	return transform(args)
}

func relaunch(binaryPath string, transformArgs func([]string) []string) error {
	cmd := exec.Command(binaryPath, resolveRelaunchArgs(transformArgs, os.Args[1:])...)
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
