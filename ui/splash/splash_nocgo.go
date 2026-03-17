//go:build darwin && !cgo

package splash

func newPlatform(_ Config) *headless { return &headless{} }
