//go:build !(windows || darwin)

package registrar

import "fmt"

type stubRegistrar struct{}

func newPlatform(_ Config) *stubRegistrar { return &stubRegistrar{} }

func (stubRegistrar) RegisterLoginItem(string) error { return nil }
func (stubRegistrar) UnregisterLoginItem() error     { return nil }
func (stubRegistrar) RegisterService(string, []string) error {
	return fmt.Errorf("registrar: not supported on this platform")
}
func (stubRegistrar) UnregisterService() error {
	return fmt.Errorf("registrar: not supported on this platform")
}
