// Splash screen visual demo.
//
// Usage:
//
//	go run ./_example/splash-demo/
//
// Shows the native splash screen with a simulated download progress,
// then dismisses it after completion. Useful for visual testing and screenshots.
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/rinktltd/go-launcher/ui/splash"
)

func main() {
	runtime.LockOSThread()

	ui := splash.New(splash.Config{
		AppName:   "NovaPulse",
		AccentHex: "#6C5CE7",
	})

	fmt.Println("showing splash...")
	ui.ShowSplash("Starting NovaPulse...")
	time.Sleep(2 * time.Second)

	ui.UpdateProgress(0, "Checking for updates...")
	time.Sleep(1 * time.Second)

	fmt.Println("simulating download...")
	for pct := 0.0; pct <= 100; pct += 2 {
		ui.UpdateProgress(pct, fmt.Sprintf("Downloading v2.1.0… %.0f%%", pct))
		time.Sleep(80 * time.Millisecond)
	}

	ui.UpdateProgress(100, "Launching...")
	time.Sleep(1 * time.Second)

	fmt.Println("hiding splash")
	ui.HideSplash()
	fmt.Println("done")
}
