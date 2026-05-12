// splash-demo opens the reference splash screen, drives a synthetic
// progress sequence, then exits. Used to visually verify spinner
// smoothness and layout on each supported platform.
package main

import (
	"fmt"
	"time"

	"github.com/razvandimescu/go-launcher/ui/splash"
)

func main() {
	ui := splash.New(splash.Config{
		AppName:   "Spinner Demo",
		AccentHex: "#2E67B2",
	})

	ui.ShowSplash("Starting up...")

	for i := 0; i <= 100; i += 5 {
		time.Sleep(300 * time.Millisecond)
		ui.UpdateProgress(float64(i), fmt.Sprintf("Working... %d%%", i))
	}

	time.Sleep(1500 * time.Millisecond)
	ui.HideSplash()
	time.Sleep(500 * time.Millisecond)
}
