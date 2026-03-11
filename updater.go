package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// bootstrapDownload downloads the latest child binary into versions/current/.
func bootstrapDownload(ctx context.Context, cfg *Config) error {
	if cfg.Fetcher == nil {
		return fmt.Errorf("no fetcher configured, cannot bootstrap")
	}

	release, err := cfg.Fetcher.LatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("fetch latest version: %w", err)
	}

	slog.Info("bootstrap: downloading child binary", "version", release.Version, "url", release.URL)

	if err := downloadToStaging(ctx, cfg, release); err != nil {
		return err
	}

	// No previous version during bootstrap — just move staging to current
	cur := currentVersionDir(cfg.DataDir)
	stg := stagingVersionDir(cfg.DataDir)

	if err := os.MkdirAll(filepath.Dir(cur), 0700); err != nil {
		return fmt.Errorf("create versions dir: %w", err)
	}

	if err := os.Rename(stg, cur); err != nil {
		return fmt.Errorf("move staging to current: %w", err)
	}

	return nil
}

// performUpdate downloads the release and rotates versions.
func performUpdate(ctx context.Context, cfg *Config, release *Release) error {
	slog.Info("updating child binary", "version", release.Version, "url", release.URL)

	if err := downloadToStaging(ctx, cfg, release); err != nil {
		return err
	}

	if err := rotateVersion(cfg.DataDir); err != nil {
		cleanStagingDir(cfg.DataDir)
		return fmt.Errorf("rotate version: %w", err)
	}

	return nil
}

// downloadToStaging downloads a release to the staging directory with checksum
// verification and progress reporting.
func downloadToStaging(ctx context.Context, cfg *Config, release *Release) error {
	stgDir := stagingVersionDir(cfg.DataDir)

	// Clean any previous staging
	os.RemoveAll(stgDir)
	if err := os.MkdirAll(stgDir, 0700); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}

	destPath := filepath.Join(stgDir, cfg.ChildBinaryName)
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}

	var w io.Writer = f
	var hasher *checksumWriter

	// Set up checksum verification if provided
	if algo, hash, ok := parseChecksum(release.Checksum); ok && algo == "sha256" {
		hasher = &checksumWriter{
			writer:   f,
			hasher:   sha256.New(),
			expected: hash,
		}
		w = hasher
	}

	progress := func(pct float64) {
		if cfg.UI != nil {
			status := fmt.Sprintf("Downloading %s...", release.Version)
			cfg.UI.UpdateProgress(pct, status)
		}
	}

	if err := cfg.Fetcher.Download(ctx, release, w, progress); err != nil {
		f.Close()
		os.RemoveAll(stgDir)
		return fmt.Errorf("download: %w", err)
	}

	if err := f.Close(); err != nil {
		os.RemoveAll(stgDir)
		return fmt.Errorf("close staging file: %w", err)
	}

	// Verify checksum
	if hasher != nil {
		actual := hex.EncodeToString(hasher.hasher.Sum(nil))
		if actual != hasher.expected {
			os.RemoveAll(stgDir)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", hasher.expected, actual)
		}
		slog.Info("checksum verified", "algo", "sha256")
	}

	// Make binary executable
	if err := os.Chmod(destPath, 0755); err != nil {
		os.RemoveAll(stgDir)
		return fmt.Errorf("chmod staging binary: %w", err)
	}

	return nil
}

type checksumWriter struct {
	writer   io.Writer
	hasher   hash.Hash
	expected string
}

func (cw *checksumWriter) Write(p []byte) (int, error) {
	n, err := cw.writer.Write(p)
	if n > 0 {
		cw.hasher.Write(p[:n])
	}
	return n, err
}

// parseChecksum splits "sha256:abc123" into ("sha256", "abc123", true).
func parseChecksum(s string) (algo, hash string, ok bool) {
	if s == "" {
		return "", "", false
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
