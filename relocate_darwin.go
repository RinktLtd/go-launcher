//go:build darwin

package launcher

import (
	"log/slog"
	"os/exec"
)

func removeQuarantine(path string) {
	if err := exec.Command("xattr", "-dr", "com.apple.quarantine", path).Run(); err != nil {
		slog.Warn("failed to remove quarantine xattr", "path", path, "error", err)
	}
}
