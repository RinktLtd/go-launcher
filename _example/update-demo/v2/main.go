// v2 of the example child. Starts up, signals healthy, does some work,
// then shuts down cleanly. This is the "updated" version.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/razvandimescu/go-launcher/child"
)

func init() {
	child.SetEnvVar("DEMO_LAUNCHER_STATE_DIR")
}

func main() {
	fmt.Println("[child v2] starting up — update successful!")
	time.Sleep(500 * time.Millisecond)

	if child.IsManaged() {
		if err := child.TouchHeartbeat(); err != nil {
			fmt.Fprintf(os.Stderr, "[child v2] heartbeat failed: %v\n", err)
		}
		fmt.Println("[child v2] heartbeat sent")
	}

	for i := 1; i <= 3; i++ {
		fmt.Printf("[child v2] working... (%d/3)\n", i)
		time.Sleep(1 * time.Second)
	}

	fmt.Println("[child v2] done, shutting down cleanly")
	if child.IsManaged() {
		child.RequestShutdown()
	}
}
