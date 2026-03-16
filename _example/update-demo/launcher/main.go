// Launcher for the update demo. Supervises v1, which requests an update to v2.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/rinktltd/go-launcher"
	"github.com/rinktltd/go-launcher/fetch"
)

func main() {
	dataDir := "/tmp/go-launcher-update-demo"

	childName := "demo-child"
	if runtime.GOOS == "windows" {
		childName += ".exe"
	}

	l := launcher.New(launcher.Config{
		AppName:         "Update Demo",
		ChildBinaryName: childName,
		DataDir:         dataDir,
		InstallDir:      "",
		EnvVarName:      "DEMO_LAUNCHER_STATE_DIR",
		Fetcher:         fetch.HTTP("unused"), // Download uses the URL from pending_update.json
		Backoff:         []time.Duration{500 * time.Millisecond, 1 * time.Second},
		CrashThreshold:  3,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("[launcher] data dir: %s\n", dataDir)
	fmt.Println("[launcher] starting supervisor loop...")
	fmt.Println()

	code := l.Run(ctx)
	fmt.Printf("\n[launcher] exiting with code %d\n", code)
	os.Exit(code)
}
