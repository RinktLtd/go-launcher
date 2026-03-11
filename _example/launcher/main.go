// Example launcher that supervises the example child binary.
//
// Usage:
//
//	# 1. Build the child binary
//	go build -o /tmp/go-launcher-example/versions/current/example-child ./_example/child/
//
//	# 2. Run the launcher
//	go run ./_example/launcher/
//
// The launcher will:
//   - Spawn the child from versions/current/
//   - Detect the heartbeat (child is healthy)
//   - Wait for the child to exit
//   - See shutdown_requested and exit cleanly
//
// Try removing the child.RequestShutdown() call from the child to see
// crash-loop detection and backoff in action.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/rinktltd/go-launcher"
)

func main() {
	dataDir := "/tmp/go-launcher-example"

	childName := "example-child"
	if runtime.GOOS == "windows" {
		childName += ".exe"
	}

	l := launcher.New(launcher.Config{
		AppName:         "Example App",
		ChildBinaryName: childName,
		DataDir:         dataDir,
		InstallDir:      "", // skip self-relocation for the example
		EnvVarName:      "EXAMPLE_LAUNCHER_STATE_DIR",

		// Fast backoff for demo purposes (production defaults: 2s, 5s, 15s)
		Backoff:           []time.Duration{500 * time.Millisecond, 1 * time.Second},
		CrashThreshold:    3,
		ProbationDuration: 30 * time.Second,
	})

	// Cancel on Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("[launcher] data dir: %s\n", dataDir)
	fmt.Printf("[launcher] child binary: versions/current/%s\n", childName)
	fmt.Println("[launcher] starting supervisor loop...")

	code := l.Run(ctx)
	fmt.Printf("[launcher] exiting with code %d\n", code)
	os.Exit(code)
}
