// Example child application managed by go-launcher.
//
// This binary signals healthy startup via heartbeat, runs for a few seconds,
// then requests a clean shutdown. Build it and place it in versions/current/
// to see the full launcher lifecycle.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rinktltd/go-launcher/child"
)

func init() {
	child.SetEnvVar("EXAMPLE_LAUNCHER_STATE_DIR")
}

func main() {
	fmt.Println("[child] starting up...")

	// Simulate initialization work
	time.Sleep(500 * time.Millisecond)

	// Signal to the launcher that we initialized successfully.
	// This dismisses the splash screen (if any) and marks us healthy.
	if child.IsManaged() {
		if err := child.TouchHeartbeat(); err != nil {
			fmt.Fprintf(os.Stderr, "[child] heartbeat failed: %v\n", err)
		}
		fmt.Println("[child] heartbeat sent to launcher")
	} else {
		fmt.Println("[child] not managed by launcher, running standalone")
	}

	// Simulate the application doing work
	for i := 1; i <= 5; i++ {
		fmt.Printf("[child] working... (%d/5)\n", i)
		time.Sleep(1 * time.Second)
	}

	// Request a clean shutdown so the launcher does not treat this as a crash.
	if child.IsManaged() {
		fmt.Println("[child] requesting clean shutdown")
		if err := child.RequestShutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "[child] shutdown request failed: %v\n", err)
		}
	}

	fmt.Println("[child] exiting")
}
