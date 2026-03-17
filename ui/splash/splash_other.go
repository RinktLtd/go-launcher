//go:build !darwin && !windows

package splash

func newPlatform(_ Config) *headless { return &headless{} }
