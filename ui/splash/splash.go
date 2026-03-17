package splash

import launcher "github.com/rinktltd/go-launcher"

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
type headless struct{}

func (headless) ShowSplash(string)              {}
func (headless) UpdateProgress(float64, string) {}
func (headless) HideSplash()                    {}
func (headless) ShowError(string)               {}

// parseHexRGB parses "#RRGGBB" into r, g, b as uint8 values.
// Returns a default blue on invalid input.
func parseHexRGB(hex string) (r, g, b uint8) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 0, 122, 255
	}
	r = hexNibble(hex[0])<<4 | hexNibble(hex[1])
	g = hexNibble(hex[2])<<4 | hexNibble(hex[3])
	b = hexNibble(hex[4])<<4 | hexNibble(hex[5])
	return
}

func hexNibble(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}
