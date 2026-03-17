//go:build windows

package splash

import (
	"math"
	"runtime"
	"sync"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ── DLL / proc handles ──

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	gdiplus  = windows.NewLazySystemDLL("gdiplus.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")

	// user32
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pPostMessage         = user32.NewProc("PostMessageW")
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pSetTimer            = user32.NewProc("SetTimer")
	pKillTimer           = user32.NewProc("KillTimer")
	pSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	pSetWindowRgn        = user32.NewProc("SetWindowRgn")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")

	// kernel32
	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")

	// gdi32
	pCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")

	// gdiplus
	pGdiplusStartup               = gdiplus.NewProc("GdiplusStartup")
	pGdiplusShutdown              = gdiplus.NewProc("GdiplusShutdown")
	pGdipCreateFromHDC            = gdiplus.NewProc("GdipCreateFromHDC")
	pGdipDeleteGraphics           = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSetSmoothingMode         = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipSetTextRenderingHint     = gdiplus.NewProc("GdipSetTextRenderingHint")
	pGdipLoadImageFromStream      = gdiplus.NewProc("GdipLoadImageFromStream")
	pGdipDisposeImage             = gdiplus.NewProc("GdipDisposeImage")
	pGdipDrawImageRectI           = gdiplus.NewProc("GdipDrawImageRectI")
	pGdipCreatePen1               = gdiplus.NewProc("GdipCreatePen1")
	pGdipDeletePen                = gdiplus.NewProc("GdipDeletePen")
	pGdipSetPenLineCap197819      = gdiplus.NewProc("GdipSetPenLineCap197819")
	pGdipDrawArc                  = gdiplus.NewProc("GdipDrawArc")
	pGdipCreateFontFamilyFromName = gdiplus.NewProc("GdipCreateFontFamilyFromName")
	pGdipDeleteFontFamily         = gdiplus.NewProc("GdipDeleteFontFamily")
	pGdipCreateFont               = gdiplus.NewProc("GdipCreateFont")
	pGdipDeleteFont               = gdiplus.NewProc("GdipDeleteFont")
	pGdipCreateSolidFill          = gdiplus.NewProc("GdipCreateSolidFill")
	pGdipDeleteBrush              = gdiplus.NewProc("GdipDeleteBrush")
	pGdipCreateStringFormat       = gdiplus.NewProc("GdipCreateStringFormat")
	pGdipDeleteStringFormat       = gdiplus.NewProc("GdipDeleteStringFormat")
	pGdipSetStringFormatAlign     = gdiplus.NewProc("GdipSetStringFormatAlign")
	pGdipDrawString               = gdiplus.NewProc("GdipDrawString")
	pGdipFillRectangleI           = gdiplus.NewProc("GdipFillRectangleI")
	pGdipTranslateWorldTransform  = gdiplus.NewProc("GdipTranslateWorldTransform")
	pGdipRotateWorldTransform     = gdiplus.NewProc("GdipRotateWorldTransform")
	pGdipResetWorldTransform      = gdiplus.NewProc("GdipResetWorldTransform")

	// ole32
	pCreateStreamOnHGlobal = ole32.NewProc("CreateStreamOnHGlobal")
)

// ── Win32 / GDI+ constants ──

const (
	wmDestroy = 0x0002
	wmPaint   = 0x000F
	wmTimer   = 0x0113
	wmEraseBG = 0x0014
	wmUser    = 0x0400

	wmApp       = wmUser + 100
	wmAppUpdate = wmApp + 1
	wmAppHide   = wmApp + 2
	wmAppError  = wmApp + 3

	wsPopup     = 0x80000000
	wsVisible   = 0x10000000
	wsExTopmost = 0x00000008
	wsExToolWin = 0x00000080

	csDropShadow = 0x00020000
	swShow       = 5
	smCxscreen   = 0
	smCyscreen   = 1
	mbIconerror  = 0x00000010
	gmemMoveable = 0x0002

	smoothingAntiAlias     = 4
	textRenderingClearType = 5
	stringAlignCenter      = 1
	matrixOrderPrepend     = 0
	fontStyleBold          = 1
	fontStyleRegular       = 0
	unitPixel              = 2
	lineCapRound           = 2
	spinnerTimerID         = 1
)

// ── Layout (matches macOS) ──

const (
	winW         = 340
	winH         = 280
	cornerRadius = 16
	logoSize     = 80
	spinnerSize  = 28
)

// ── Colors (ARGB for GDI+) ──

const (
	colorWhiteBg   = 0xFFFFFFFF
	colorLightGray = 0xFFE0E0E0
	colorTitle     = 0xFF1A1A1A
	colorStatus    = 0xFF888888
)

// ── Win32 structs ──

type wndClassExW struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaintL    int32
	rcPaintT    int32
	rcPaintR    int32
	rcPaintB    int32
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

type gdiplusStartupInput struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}

type rectF struct {
	X, Y, Width, Height float32
}

// ── Command types ──

const (
	cmdUpdate = iota
	cmdHide
	cmdError
)

type splashCmd struct {
	kind    int
	percent float64
	text    string
}

// ── winSplash ──

type winSplash struct {
	hwnd uintptr
	cmds chan splashCmd

	cfg        Config
	accentARGB uint32

	mu           sync.Mutex
	statusText   string
	progressPct  float64
	spinnerAngle float32

	gdipToken  uintptr
	gdipImage  uintptr
	titleFont  uintptr
	statusFont uintptr
	fontFamily uintptr
	strFormat  uintptr
}

func newPlatform(cfg Config) *winSplash {
	r, g, b := parseHexRGB(cfg.AccentHex)
	argb := uint32(0xFF000000) | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	return &winSplash{
		cmds:       make(chan splashCmd, 16),
		cfg:        cfg,
		accentARGB: argb,
	}
}

func (s *winSplash) ShowSplash(status string) {
	s.mu.Lock()
	s.statusText = status
	s.mu.Unlock()
	go s.run()
}

func (s *winSplash) UpdateProgress(percent float64, status string) {
	select {
	case s.cmds <- splashCmd{kind: cmdUpdate, percent: percent, text: status}:
	default:
	}
}

func (s *winSplash) HideSplash() {
	select {
	case s.cmds <- splashCmd{kind: cmdHide}:
	default:
	}
}

func (s *winSplash) ShowError(msg string) {
	select {
	case s.cmds <- splashCmd{kind: cmdError, text: msg}:
	default:
	}
}

func f32bits(f float32) uintptr {
	return uintptr(math.Float32bits(f))
}

func gdipFillRect(graphics uintptr, color uint32, x, y, w, h int) {
	var brush uintptr
	pGdipCreateSolidFill.Call(uintptr(color), uintptr(unsafe.Pointer(&brush)))
	pGdipFillRectangleI.Call(graphics, brush, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
	pGdipDeleteBrush.Call(brush)
}

func loadPNGFromMemory(data []byte) uintptr {
	hMem, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if hMem == 0 {
		return 0
	}
	ptr, _, _ := pGlobalLock.Call(hMem)
	if ptr == 0 {
		return 0
	}
	copy((*[1 << 30]byte)(unsafe.Pointer(ptr))[:len(data)], data)
	pGlobalUnlock.Call(hMem)

	var stream uintptr
	pCreateStreamOnHGlobal.Call(hMem, 1, uintptr(unsafe.Pointer(&stream)))
	if stream == 0 {
		return 0
	}

	var img uintptr
	pGdipLoadImageFromStream.Call(stream, uintptr(unsafe.Pointer(&img)))
	return img
}

func (s *winSplash) initGDIPlus() {
	input := gdiplusStartupInput{GdiplusVersion: 1}
	pGdiplusStartup.Call(
		uintptr(unsafe.Pointer(&s.gdipToken)),
		uintptr(unsafe.Pointer(&input)),
		0,
	)

	if len(s.cfg.Logo) > 0 {
		s.gdipImage = loadPNGFromMemory(s.cfg.Logo)
	}

	familyName := windows.StringToUTF16Ptr("Segoe UI")
	pGdipCreateFontFamilyFromName.Call(
		uintptr(unsafe.Pointer(familyName)), 0,
		uintptr(unsafe.Pointer(&s.fontFamily)),
	)

	pGdipCreateFont.Call(s.fontFamily, f32bits(17.0), fontStyleBold, unitPixel,
		uintptr(unsafe.Pointer(&s.titleFont)))

	pGdipCreateFont.Call(s.fontFamily, f32bits(12.0), fontStyleRegular, unitPixel,
		uintptr(unsafe.Pointer(&s.statusFont)))

	pGdipCreateStringFormat.Call(0, 0, uintptr(unsafe.Pointer(&s.strFormat)))
	pGdipSetStringFormatAlign.Call(s.strFormat, stringAlignCenter)
}

func (s *winSplash) shutdownGDIPlus() {
	if s.strFormat != 0 {
		pGdipDeleteStringFormat.Call(s.strFormat)
	}
	if s.titleFont != 0 {
		pGdipDeleteFont.Call(s.titleFont)
	}
	if s.statusFont != 0 {
		pGdipDeleteFont.Call(s.statusFont)
	}
	if s.fontFamily != 0 {
		pGdipDeleteFontFamily.Call(s.fontFamily)
	}
	if s.gdipImage != 0 {
		pGdipDisposeImage.Call(s.gdipImage)
	}
	if s.gdipToken != 0 {
		pGdiplusShutdown.Call(s.gdipToken)
	}
}

func (s *winSplash) paint(hdc uintptr) {
	var graphics uintptr
	pGdipCreateFromHDC.Call(hdc, uintptr(unsafe.Pointer(&graphics)))
	if graphics == 0 {
		return
	}
	defer pGdipDeleteGraphics.Call(graphics)

	pGdipSetSmoothingMode.Call(graphics, smoothingAntiAlias)
	pGdipSetTextRenderingHint.Call(graphics, textRenderingClearType)

	gdipFillRect(graphics, colorWhiteBg, 0, 0, winW, winH)

	if s.gdipImage != 0 {
		logoX := (winW - logoSize) / 2
		pGdipDrawImageRectI.Call(graphics, s.gdipImage,
			uintptr(logoX), 30, logoSize, logoSize)
	}

	titleY := float32(30 + logoSize + 8)
	s.drawCenteredText(graphics, s.cfg.AppName, s.titleFont, colorTitle, titleY, 26)

	s.drawSpinner(graphics)

	s.mu.Lock()
	pct := s.progressPct
	status := s.statusText
	s.mu.Unlock()

	if pct > 0 {
		barX, barY, barW, barH := 50, 204, winW-100, 4
		gdipFillRect(graphics, colorLightGray, barX, barY, barW, barH)
		fillW := int(float64(barW) * pct / 100.0)
		if fillW > 0 {
			gdipFillRect(graphics, s.accentARGB, barX, barY, fillW, barH)
		}
	}

	if status != "" {
		s.drawCenteredText(graphics, status, s.statusFont, colorStatus, 234, 20)
	}
}

func (s *winSplash) drawCenteredText(graphics uintptr, text string, font uintptr, color uint32, y, height float32) {
	textPtr := windows.StringToUTF16Ptr(text)
	textLen := utf8.RuneCountInString(text)

	var brush uintptr
	pGdipCreateSolidFill.Call(uintptr(color), uintptr(unsafe.Pointer(&brush)))
	defer pGdipDeleteBrush.Call(brush)

	rect := rectF{X: 0, Y: y, Width: winW, Height: height}
	pGdipDrawString.Call(graphics,
		uintptr(unsafe.Pointer(textPtr)), uintptr(textLen),
		font, uintptr(unsafe.Pointer(&rect)), s.strFormat, brush)
}

func (s *winSplash) drawSpinner(graphics uintptr) {
	cx := float32(winW) / 2
	cy := float32(30+logoSize+8+26+20) + float32(spinnerSize)/2
	r := float32(spinnerSize-3) / 2

	s.mu.Lock()
	angle := s.spinnerAngle
	s.mu.Unlock()

	var trackPen uintptr
	pGdipCreatePen1.Call(colorLightGray, f32bits(2.5), unitPixel, uintptr(unsafe.Pointer(&trackPen)))
	pGdipDrawArc.Call(graphics, trackPen,
		f32bits(cx-r), f32bits(cy-r), f32bits(2*r), f32bits(2*r),
		f32bits(0), f32bits(360))
	pGdipDeletePen.Call(trackPen)

	var arcPen uintptr
	pGdipCreatePen1.Call(uintptr(s.accentARGB), f32bits(2.5), unitPixel, uintptr(unsafe.Pointer(&arcPen)))
	pGdipSetPenLineCap197819.Call(arcPen, lineCapRound, lineCapRound, 0)

	pGdipTranslateWorldTransform.Call(graphics, f32bits(cx), f32bits(cy), matrixOrderPrepend)
	pGdipRotateWorldTransform.Call(graphics, f32bits(angle), matrixOrderPrepend)
	pGdipTranslateWorldTransform.Call(graphics, f32bits(-cx), f32bits(-cy), matrixOrderPrepend)

	pGdipDrawArc.Call(graphics, arcPen,
		f32bits(cx-r), f32bits(cy-r), f32bits(2*r), f32bits(2*r),
		f32bits(0), f32bits(270))

	pGdipResetWorldTransform.Call(graphics)
	pGdipDeletePen.Call(arcPen)
}

func (s *winSplash) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	s.initGDIPlus()
	defer s.shutdownGDIPlus()

	hInstance, _, _ := pGetModuleHandleW.Call(0)
	className := windows.StringToUTF16Ptr("GoLauncherSplash")

	wndProc := windows.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmPaint:
			var ps paintStruct
			hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			s.paint(hdc)
			pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			return 0
		case wmEraseBG:
			return 1
		case wmTimer:
			if wParam == spinnerTimerID {
				s.mu.Lock()
				s.spinnerAngle -= 12
				if s.spinnerAngle <= -360 {
					s.spinnerAngle += 360
				}
				s.mu.Unlock()
				pInvalidateRect.Call(hwnd, 0, 0)
			}
			return 0
		case wmDestroy:
			return 0
		}
		ret, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	})

	wcx := wndClassExW{
		size:      uint32(unsafe.Sizeof(wndClassExW{})),
		style:     csDropShadow,
		wndProc:   wndProc,
		instance:  windows.Handle(hInstance),
		className: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

	screenW, _, _ := pGetSystemMetrics.Call(smCxscreen)
	screenH, _, _ := pGetSystemMetrics.Call(smCyscreen)
	x := (screenW - winW) / 2
	y := (screenH - winH) / 2

	hwnd, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExToolWin,
		uintptr(unsafe.Pointer(className)), 0,
		wsPopup|wsVisible,
		x, y, winW, winH,
		0, 0, hInstance, 0,
	)
	s.hwnd = hwnd

	rgn, _, _ := pCreateRoundRectRgn.Call(0, 0, winW+1, winH+1, cornerRadius*2, cornerRadius*2)
	pSetWindowRgn.Call(hwnd, rgn, 1)

	pSetWindowLongPtrW.Call(hwnd, ^uintptr(3), wndProc) // GWLP_WNDPROC = -4

	pShowWindow.Call(hwnd, swShow)
	pSetForegroundWindow.Call(hwnd)

	pSetTimer.Call(hwnd, spinnerTimerID, 33, 0)

	go func() {
		for cmd := range s.cmds {
			switch cmd.kind {
			case cmdUpdate:
				s.mu.Lock()
				s.progressPct = cmd.percent
				s.statusText = cmd.text
				s.mu.Unlock()
				pPostMessage.Call(hwnd, wmAppUpdate, 0, 0)
			case cmdHide:
				pPostMessage.Call(hwnd, wmAppHide, 0, 0)
				return
			case cmdError:
				s.mu.Lock()
				s.statusText = cmd.text
				s.mu.Unlock()
				pPostMessage.Call(hwnd, wmAppError, 0, 0)
				return
			}
		}
	}()

	var m winMsg
	for {
		ret, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		switch m.message {
		case wmAppUpdate:
			pInvalidateRect.Call(hwnd, 0, 0)
		case wmAppHide:
			pKillTimer.Call(hwnd, spinnerTimerID)
			pDestroyWindow.Call(hwnd)
			return
		case wmAppError:
			pKillTimer.Call(hwnd, spinnerTimerID)
			pDestroyWindow.Call(hwnd)
			s.mu.Lock()
			errText := s.statusText
			s.mu.Unlock()
			errPtr := windows.StringToUTF16Ptr(errText)
			titlePtr := windows.StringToUTF16Ptr(s.cfg.AppName)
			pMessageBoxW.Call(0, uintptr(unsafe.Pointer(errPtr)),
				uintptr(unsafe.Pointer(titlePtr)), mbIconerror)
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
