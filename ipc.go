package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	heartbeatFile       = "heartbeat"
	shutdownFile        = "shutdown_requested"
	pendingUpdateFile   = "pending_update.json"
)

// heartbeatTouchedAfter returns true if the heartbeat file's mtime is after t.
func heartbeatTouchedAfter(dataDir string, t time.Time) bool {
	info, err := os.Stat(filepath.Join(dataDir, heartbeatFile))
	if err != nil {
		return false
	}
	return info.ModTime().After(t)
}

// shutdownRequested returns true if the shutdown_requested file exists.
func shutdownRequested(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, shutdownFile))
	return err == nil
}

// deleteShutdownFile removes the shutdown_requested file.
func deleteShutdownFile(dataDir string) {
	os.Remove(filepath.Join(dataDir, shutdownFile))
}

// readPendingUpdate reads and parses pending_update.json. Returns nil if
// the file does not exist or is malformed.
func readPendingUpdate(dataDir string) *Release {
	data, err := os.ReadFile(filepath.Join(dataDir, pendingUpdateFile))
	if err != nil {
		return nil
	}

	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	if r.URL == "" {
		return nil
	}
	return &r
}

// deletePendingUpdate removes pending_update.json and its tmp file.
func deletePendingUpdate(dataDir string) {
	os.Remove(filepath.Join(dataDir, pendingUpdateFile))
	os.Remove(filepath.Join(dataDir, pendingUpdateFile+".tmp"))
}

// deleteHeartbeat removes the heartbeat file.
func deleteHeartbeat(dataDir string) {
	os.Remove(filepath.Join(dataDir, heartbeatFile))
}

// writePendingUpdate atomically writes pending_update.json.
func writePendingUpdate(dataDir string, r *Release) error {
	return atomicWriteJSON(filepath.Join(dataDir, pendingUpdateFile), r)
}

// writeShutdownRequested creates the shutdown_requested flag file.
func writeShutdownRequested(dataDir string) error {
	path := filepath.Join(dataDir, shutdownFile)
	return os.WriteFile(path, []byte(""), 0600)
}
