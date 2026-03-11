package launcher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// testChild builds a small test binary that behaves as instructed via env vars.
func testChild(t *testing.T, dir string) string {
	t.Helper()
	name := "test-child"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	src := filepath.Join(dir, "main.go")
	bin := filepath.Join(dir, name)

	os.WriteFile(src, []byte(`package main

import (
	"os"
	"time"
)

func main() {
	stateDir := os.Getenv("TEST_LAUNCHER_STATE_DIR")

	// Touch heartbeat if instructed
	if os.Getenv("CHILD_HEARTBEAT") == "1" && stateDir != "" {
		f, _ := os.Create(stateDir + "/heartbeat")
		f.Close()
	}

	// Write shutdown_requested if instructed
	if os.Getenv("CHILD_SHUTDOWN") == "1" && stateDir != "" {
		os.WriteFile(stateDir + "/shutdown_requested", []byte(""), 0600)
	}

	// Sleep if instructed
	if d := os.Getenv("CHILD_SLEEP"); d != "" {
		dur, _ := time.ParseDuration(d)
		time.Sleep(dur)
	}

	// Exit with code
	if os.Getenv("CHILD_EXIT_CODE") == "1" {
		os.Exit(1)
	}
}
`), 0644)

	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build test child: %v\n%s", err, out)
	}

	return name
}

func setupTestLauncher(t *testing.T) (cfg Config, dataDir string, cleanup func()) {
	t.Helper()
	dataDir = t.TempDir()
	childDir := t.TempDir()

	childName := testChild(t, childDir)

	// Place child binary in versions/current/
	curDir := filepath.Join(dataDir, "versions", "current")
	os.MkdirAll(curDir, 0700)

	src := filepath.Join(childDir, childName)
	dst := filepath.Join(curDir, childName)
	data, _ := os.ReadFile(src)
	os.WriteFile(dst, data, 0755)

	cfg = Config{
		AppName:           "test-app",
		ChildBinaryName:   childName,
		DataDir:           dataDir,
		InstallDir:        "", // skip relocation in tests
		EnvVarName:        "TEST_LAUNCHER_STATE_DIR",
		Backoff:           []time.Duration{100 * time.Millisecond, 200 * time.Millisecond},
		CrashThreshold:    3,
		ProbationDuration: 500 * time.Millisecond,
		KillTimeout:       2 * time.Second,
	}

	return cfg, dataDir, func() {}
}

func TestCleanShutdown(t *testing.T) {
	cfg, _, cleanup := setupTestLauncher(t)
	defer cleanup()

	// Child will touch heartbeat and write shutdown_requested
	cfg.ChildArgs = nil
	os.Setenv("CHILD_HEARTBEAT", "1")
	os.Setenv("CHILD_SHUTDOWN", "1")
	os.Setenv("CHILD_EXIT_CODE", "0")
	defer func() {
		os.Unsetenv("CHILD_HEARTBEAT")
		os.Unsetenv("CHILD_SHUTDOWN")
		os.Unsetenv("CHILD_EXIT_CODE")
	}()

	l := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := l.Run(ctx)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestCrashLoopExits(t *testing.T) {
	cfg, _, cleanup := setupTestLauncher(t)
	defer cleanup()

	os.Setenv("CHILD_EXIT_CODE", "1")
	defer os.Unsetenv("CHILD_EXIT_CODE")

	l := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	code := l.Run(ctx)
	if code != 1 {
		t.Errorf("expected exit code 1 after crash loop, got %d", code)
	}

	// Verify crash count reached threshold
	state, _ := loadState(cfg.DataDir)
	if state.CrashCount < cfg.CrashThreshold {
		t.Errorf("expected crash count >= %d, got %d", cfg.CrashThreshold, state.CrashCount)
	}
}

func TestRestartOnUnexpectedExit0(t *testing.T) {
	cfg, dataDir, cleanup := setupTestLauncher(t)
	defer cleanup()

	// Child exits 0 without shutdown_requested — should be treated as crash
	os.Setenv("CHILD_EXIT_CODE", "0")
	defer os.Unsetenv("CHILD_EXIT_CODE")

	l := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	code := l.Run(ctx)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	state, _ := loadState(dataDir)
	if state.CrashCount < cfg.CrashThreshold {
		t.Errorf("expected crash count >= %d, got %d", cfg.CrashThreshold, state.CrashCount)
	}
}

func TestLockfilePreventsDoubleStart(t *testing.T) {
	dataDir := t.TempDir()

	lf := newLockfile(dataDir)
	if err := lf.Acquire(); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer lf.Release()

	lf2 := newLockfile(dataDir)
	err := lf2.Acquire()
	if err == nil {
		t.Error("expected second acquire to fail")
	}
}

func TestStaleLockfileRecovery(t *testing.T) {
	dataDir := t.TempDir()

	// Write a lockfile with a non-existent PID
	lockPath := filepath.Join(dataDir, lockFileName)
	os.WriteFile(lockPath, []byte("999999999"), 0600)

	lf := newLockfile(dataDir)
	if err := lf.Acquire(); err != nil {
		t.Fatalf("should acquire stale lockfile, got: %v", err)
	}
	lf.Release()
}

func TestStateAtomicWriteAndLoad(t *testing.T) {
	dataDir := t.TempDir()

	s := &State{
		CurrentVersion:  "1.0.0",
		PreviousVersion: "0.9.0",
		CrashCount:      2,
	}

	if err := saveState(dataDir, s); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadState(dataDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.CurrentVersion != "1.0.0" || loaded.PreviousVersion != "0.9.0" || loaded.CrashCount != 2 {
		t.Errorf("loaded state mismatch: %+v", loaded)
	}
}

func TestCorruptedStateReturnsDefaults(t *testing.T) {
	dataDir := t.TempDir()
	os.WriteFile(filepath.Join(dataDir, stateFileName), []byte("not json"), 0600)

	s, err := loadState(dataDir)
	if err != nil {
		t.Fatalf("expected no error on corrupt state, got: %v", err)
	}
	if s.CurrentVersion != "" || s.CrashCount != 0 {
		t.Errorf("expected empty defaults, got: %+v", s)
	}
}

func TestVersionRotation(t *testing.T) {
	dataDir := t.TempDir()
	os.MkdirAll(filepath.Join(dataDir, "versions", "current"), 0700)
	os.MkdirAll(filepath.Join(dataDir, "versions", "staging"), 0700)

	// Place marker files
	os.WriteFile(filepath.Join(dataDir, "versions", "current", "old"), []byte("old"), 0600)
	os.WriteFile(filepath.Join(dataDir, "versions", "staging", "new"), []byte("new"), 0600)

	if err := rotateVersion(dataDir); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// current should have the new file
	if _, err := os.Stat(filepath.Join(dataDir, "versions", "current", "new")); err != nil {
		t.Error("expected 'new' in current after rotation")
	}

	// previous should have the old file
	if _, err := os.Stat(filepath.Join(dataDir, "versions", "previous", "old")); err != nil {
		t.Error("expected 'old' in previous after rotation")
	}
}

func TestRollback(t *testing.T) {
	dataDir := t.TempDir()
	os.MkdirAll(filepath.Join(dataDir, "versions", "current"), 0700)
	os.MkdirAll(filepath.Join(dataDir, "versions", "previous"), 0700)

	os.WriteFile(filepath.Join(dataDir, "versions", "current", "bad"), []byte("bad"), 0600)
	os.WriteFile(filepath.Join(dataDir, "versions", "previous", "good"), []byte("good"), 0600)

	if err := rollback(dataDir); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// current should have the good file
	if _, err := os.Stat(filepath.Join(dataDir, "versions", "current", "good")); err != nil {
		t.Error("expected 'good' in current after rollback")
	}

	// previous should have the bad file
	if _, err := os.Stat(filepath.Join(dataDir, "versions", "previous", "bad")); err != nil {
		t.Error("expected 'bad' in previous after rollback")
	}

	// rollback-tmp should not exist
	if _, err := os.Stat(filepath.Join(dataDir, "versions", "rollback-tmp")); !os.IsNotExist(err) {
		t.Error("rollback-tmp should be cleaned up")
	}
}

func TestInterruptedSwapRecovery(t *testing.T) {
	t.Run("rollback-tmp exists, current missing", func(t *testing.T) {
		dataDir := t.TempDir()
		os.MkdirAll(filepath.Join(dataDir, "versions", "rollback-tmp"), 0700)
		os.WriteFile(filepath.Join(dataDir, "versions", "rollback-tmp", "file"), []byte("data"), 0600)

		recoverInterruptedSwap(dataDir)

		if _, err := os.Stat(filepath.Join(dataDir, "versions", "current", "file")); err != nil {
			t.Error("expected rollback-tmp recovered to current")
		}
	})

	t.Run("rollback-tmp exists, current exists", func(t *testing.T) {
		dataDir := t.TempDir()
		os.MkdirAll(filepath.Join(dataDir, "versions", "rollback-tmp"), 0700)
		os.MkdirAll(filepath.Join(dataDir, "versions", "current"), 0700)

		recoverInterruptedSwap(dataDir)

		if _, err := os.Stat(filepath.Join(dataDir, "versions", "rollback-tmp")); !os.IsNotExist(err) {
			t.Error("expected rollback-tmp to be cleaned")
		}
	})
}

func TestIPCFiles(t *testing.T) {
	dataDir := t.TempDir()

	// Heartbeat
	if heartbeatTouchedAfter(dataDir, time.Now().Add(-time.Hour)) {
		t.Error("heartbeat should not exist yet")
	}

	os.WriteFile(filepath.Join(dataDir, heartbeatFile), []byte(""), 0600)
	if !heartbeatTouchedAfter(dataDir, time.Now().Add(-time.Hour)) {
		t.Error("heartbeat should exist now")
	}

	// Shutdown
	if shutdownRequested(dataDir) {
		t.Error("shutdown should not be requested yet")
	}

	writeShutdownRequested(dataDir)
	if !shutdownRequested(dataDir) {
		t.Error("shutdown should be requested now")
	}

	deleteShutdownFile(dataDir)
	if shutdownRequested(dataDir) {
		t.Error("shutdown should be cleared")
	}

	// Pending update
	if r := readPendingUpdate(dataDir); r != nil {
		t.Error("no pending update should exist")
	}

	writePendingUpdate(dataDir, &Release{Version: "2.0.0", URL: "https://example.com/v2"})
	r := readPendingUpdate(dataDir)
	if r == nil || r.Version != "2.0.0" {
		t.Errorf("expected pending update v2.0.0, got: %+v", r)
	}

	deletePendingUpdate(dataDir)
	if r := readPendingUpdate(dataDir); r != nil {
		t.Error("pending update should be deleted")
	}
}

func TestChecksumParsing(t *testing.T) {
	tests := []struct {
		input    string
		wantAlgo string
		wantHash string
		wantOK   bool
	}{
		{"sha256:abc123", "sha256", "abc123", true},
		{"", "", "", false},
		{"nocolon", "", "", false},
		{"sha256:", "", "", false},
	}

	for _, tt := range tests {
		algo, hash, ok := parseChecksum(tt.input)
		if algo != tt.wantAlgo || hash != tt.wantHash || ok != tt.wantOK {
			t.Errorf("parseChecksum(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, algo, hash, ok, tt.wantAlgo, tt.wantHash, tt.wantOK)
		}
	}
}
