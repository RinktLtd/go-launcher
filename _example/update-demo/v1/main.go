// v1 of the example child. Starts up, signals healthy, then "discovers"
// that v2 is available and requests an update.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rinktltd/go-launcher/child"
)

func init() {
	child.SetEnvVar("DEMO_LAUNCHER_STATE_DIR")
}

func main() {
	fmt.Println("[child v1] starting up...")
	time.Sleep(500 * time.Millisecond)

	if child.IsManaged() {
		if err := child.TouchHeartbeat(); err != nil {
			fmt.Fprintf(os.Stderr, "[child v1] heartbeat failed: %v\n", err)
		}
		fmt.Println("[child v1] heartbeat sent")
	}

	fmt.Println("[child v1] working...")
	time.Sleep(2 * time.Second)

	// "Discover" that v2 is available. In production this would be an API call,
	// GitHub check, etc. Here we read the URL and checksum from env vars that
	// the demo script sets.
	url := os.Getenv("DEMO_UPDATE_URL")
	checksum := os.Getenv("DEMO_UPDATE_CHECKSUM")

	if child.IsManaged() && url != "" {
		fmt.Println("[child v1] update available! requesting update to v2...")
		if err := child.RequestUpdate("2.0.0", url, checksum); err != nil {
			fmt.Fprintf(os.Stderr, "[child v1] update request failed: %v\n", err)
			child.RequestShutdown()
		}
	} else {
		fmt.Println("[child v1] no update available, shutting down")
		if child.IsManaged() {
			child.RequestShutdown()
		}
	}

	fmt.Println("[child v1] exiting")
}
