//go:build darwin || windows

package splash

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
