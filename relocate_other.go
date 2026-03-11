//go:build !darwin

package launcher

func removeQuarantine(_ string) {
	// no-op on non-macOS platforms
}
