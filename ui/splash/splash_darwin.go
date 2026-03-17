//go:build darwin && cgo

package splash

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include <stdlib.h>
#include "splash_darwin.h"
*/
import "C"
import "unsafe"

type darwinSplash struct {
	appName string
}

func newPlatform(cfg Config) *darwinSplash {
	if len(cfg.Logo) > 0 {
		C.SplashSetIcon((*C.char)(unsafe.Pointer(&cfg.Logo[0])), C.int(len(cfg.Logo)))
	}

	ct := C.CString(cfg.AppName)
	defer C.free(unsafe.Pointer(ct))
	C.SplashSetTitle(ct)

	r, g, b := parseHexRGB(cfg.AccentHex)
	C.SplashSetColor(C.float(float32(r)/255), C.float(float32(g)/255), C.float(float32(b)/255))

	return &darwinSplash{appName: cfg.AppName}
}

func (s *darwinSplash) ShowSplash(status string) {
	cs := C.CString(status)
	defer C.free(unsafe.Pointer(cs))
	C.SplashShow(cs)
}

func (s *darwinSplash) UpdateProgress(percent float64, status string) {
	cs := C.CString(status)
	defer C.free(unsafe.Pointer(cs))
	C.SplashUpdate(C.double(percent), cs)
}

func (s *darwinSplash) HideSplash() {
	C.SplashHide()
}

func (s *darwinSplash) ShowError(msg string) {
	cs := C.CString(msg)
	defer C.free(unsafe.Pointer(cs))
	C.SplashError(cs)
}
