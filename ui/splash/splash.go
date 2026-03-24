package splash

import launcher "github.com/razvandimescu/go-launcher"

// Config configures the reference splash screen.
type Config struct {
	AppName   string // window title and error dialog heading
	Logo      []byte // PNG image bytes; nil = no logo
	AccentHex string // hex color for spinner and progress bar (e.g. "#2E67B2"); default "#007AFF"
}

func (c *Config) defaults() {
	if c.AppName == "" {
		c.AppName = "Application"
	}
	if c.AccentHex == "" {
		c.AccentHex = "#007AFF"
	}
}

// New returns a launcher.UI backed by the native splash screen for the
// current OS. On unsupported platforms (Linux) or when CGo is unavailable
// on macOS, it returns a silent no-op implementation.
func New(cfg Config) launcher.UI {
	cfg.defaults()
	return newPlatform(cfg)
}

// headless is a silent no-op UI for unsupported platforms.
// Used by splash_nocgo.go and splash_other.go via build tags.
type headless struct{}

var _ launcher.UI = (*headless)(nil)

func (headless) ShowSplash(string)              {}
func (headless) UpdateProgress(float64, string) {}
func (headless) HideSplash()                    {}
func (headless) ShowError(string)               {}
