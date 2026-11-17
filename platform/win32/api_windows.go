package win32

import (
	"fmt"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// DWMWA_USE_IMMERSIVE_DARK_MODE is the attribute behind a dark title bar.
const DWMWA_USE_IMMERSIVE_DARK_MODE = 20

// HWND is a Win32 window handle.
type HWND = platform.HWND

// Message and style constants keep their Win32 header names so they can be
// searched for (CODING_STYLE §8).
const (
	WM_DESTROY              = 0x0002
	WM_SIZE                 = 0x0005
	WM_PAINT                = 0x000F
	WM_CLOSE                = 0x0010
	WM_ERASEBKGND           = 0x0014
	WM_SETTINGCHANGE        = 0x001A
	WM_SETFOCUS             = 0x0007
	WM_KILLFOCUS            = 0x0008
	WM_SETCURSOR            = 0x0020
	WM_KEYDOWN              = 0x0100
	WM_KEYUP                = 0x0101
	WM_CHAR                 = 0x0102
	WM_SYSKEYDOWN           = 0x0104
	WM_SYSKEYUP             = 0x0105
	WM_SYSCHAR              = 0x0106
	WM_IME_STARTCOMPOSITION = 0x010D
	WM_IME_ENDCOMPOSITION   = 0x010E
	WM_IME_COMPOSITION      = 0x010F
	WM_TIMER                = 0x0113
	WM_MOUSEMOVE            = 0x0200
	WM_LBUTTONDOWN          = 0x0201
	WM_LBUTTONUP            = 0x0202
	WM_RBUTTONDOWN          = 0x0204
	WM_RBUTTONUP            = 0x0205
	WM_MBUTTONDOWN          = 0x0207
	WM_MBUTTONUP            = 0x0208
	WM_MOUSEWHEEL           = 0x020A
	WM_XBUTTONDOWN          = 0x020B
	WM_XBUTTONUP            = 0x020C
	WM_MOUSEHWHEEL          = 0x020E
	WM_CAPTURECHANGED       = 0x0215
	WM_IME_SETCONTEXT       = 0x0281
	WM_MOUSELEAVE           = 0x02A3
	WM_ENTERMENULOOP        = 0x0211
	WM_EXITMENULOOP         = 0x0212
	WM_ENTERSIZEMOVE        = 0x0231
	WM_EXITSIZEMOVE         = 0x0232
	WM_DPICHANGED           = 0x02E0
	WM_USER                 = 0x0400

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	CW_USEDEFAULT       = 0x80000000
	SW_SHOWNORMAL       = 1
	SW_SHOW             = 5
	IDC_ARROW           = 32512
	IDC_IBEAM           = 32513
	IDC_HAND            = 32649
	IDC_SIZEWE          = 32644
	IDC_SIZENS          = 32645
	HTCLIENT            = 1
	WHEEL_DELTA         = 120
	TME_LEAVE           = 0x00000002
	CF_UNICODETEXT      = 13
	GMEM_MOVEABLE       = 0x0002

	VK_BACK    = 0x08
	VK_TAB     = 0x09
	VK_RETURN  = 0x0D
	VK_SHIFT   = 0x10
	VK_CONTROL = 0x11
	VK_MENU    = 0x12
	VK_ESCAPE  = 0x1B
	VK_SPACE   = 0x20
	VK_PRIOR   = 0x21
	VK_NEXT    = 0x22
	VK_END     = 0x23
	VK_HOME    = 0x24
	VK_LEFT    = 0x25
	VK_UP      = 0x26
	VK_RIGHT   = 0x27
	VK_DOWN    = 0x28
	VK_INSERT  = 0x2D
	VK_DELETE  = 0x2E
	VK_APPS    = 0x5D
	VK_F1      = 0x70
	VK_F4      = 0x73
	VK_F12     = 0x7B

	VK_MULTIPLY = 0x6A
	VK_ADD      = 0x6B
	VK_SUBTRACT = 0x6D
	VK_DIVIDE   = 0x6F

	// The OEM keys type what the layout says. The names are of the US layout.
	VK_OEM_1      = 0xBA
	VK_OEM_PLUS   = 0xBB
	VK_OEM_COMMA  = 0xBC
	VK_OEM_MINUS  = 0xBD
	VK_OEM_PERIOD = 0xBE
	VK_OEM_2      = 0xBF
	VK_OEM_3      = 0xC0
	VK_OEM_4      = 0xDB
	VK_OEM_5      = 0xDC
	VK_OEM_6      = 0xDD
	VK_OEM_7      = 0xDE
	VK_OEM_8      = 0xDF
	VK_OEM_102    = 0xE2

	MAPVK_VK_TO_CHAR = 2

	GCS_COMPSTR                 = 0x0008
	GCS_COMPATTR                = 0x0010
	GCS_COMPCLAUSE              = 0x0020
	GCS_CURSORPOS               = 0x0080
	GCS_RESULTSTR               = 0x0800
	CFS_POINT                   = 0x0002
	SHIFTJIS_CHARSET            = 128
	CFS_EXCLUDE                 = 0x0080
	IACE_DEFAULT                = 0x0010
	NI_COMPOSITIONSTR           = 0x0015
	CPS_CANCEL                  = 0x0004
	ISC_SHOWUICOMPOSITIONWINDOW = 0x80000000

	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	PM_REMOVE           = 0x0001
	QS_ALLINPUT         = 0x04FF
	MWMO_INPUTAVAILABLE = 0x0004
	INFINITE            = 0xFFFFFFFF
	WAIT_TIMEOUT        = 258

	USER_DEFAULT_SCREEN_DPI = 96

	// HWND_MESSAGE is (HWND)-3.
	HWND_MESSAGE = ^uintptr(2)
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (DPI_AWARENESS_CONTEXT)-4.
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3)
)

// POINT is the Win32 POINT structure.
type POINT struct {
	x, y int32
}

// RECT is the Win32 RECT structure.
type RECT struct {
	left, top, right, bottom int32
}

// MSG is the Win32 MSG structure.
type MSG struct {
	hwnd     HWND
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       POINT
	lPrivate uint32
}

// TRACKMOUSEEVENT is the Win32 TRACKMOUSEEVENT structure.
type TRACKMOUSEEVENT struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   HWND
	dwHoverTime uint32
}

// COMPOSITIONFORM is the Win32 COMPOSITIONFORM structure.
type COMPOSITIONFORM struct {
	dwStyle      uint32
	ptCurrentPos POINT
	rcArea       RECT
}

// LOGFONTW is the Win32 LOGFONTW structure.
type LOGFONTW struct {
	lfHeight         int32
	lfWidth          int32
	lfEscapement     int32
	lfOrientation    int32
	lfWeight         int32
	lfItalic         byte
	lfUnderline      byte
	lfStrikeOut      byte
	lfCharSet        byte
	lfOutPrecision   byte
	lfClipPrecision  byte
	lfQuality        byte
	lfPitchAndFamily byte
	lfFaceName       [32]uint16
}

// CANDIDATEFORM is the Win32 CANDIDATEFORM structure.
type CANDIDATEFORM struct {
	dwIndex      uint32
	dwStyle      uint32
	ptCurrentPos POINT
	rcArea       RECT
}

// WNDCLASSEXW is the Win32 WNDCLASSEXW structure.
type WNDCLASSEXW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	imm32    = windows.NewLazySystemDLL("imm32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	procAdjustWindowRectExForDpi      = user32.NewProc("AdjustWindowRectExForDpi")
	procSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procKillTimer                     = user32.NewProc("KillTimer")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procMsgWaitForMultipleObjectsEx   = user32.NewProc("MsgWaitForMultipleObjectsEx")
	procPeekMessageW                  = user32.NewProc("PeekMessageW")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetTimer                      = user32.NewProc("SetTimer")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procSetWindowTextW                = user32.NewProc("SetWindowTextW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procUnregisterClassW              = user32.NewProc("UnregisterClassW")
	procValidateRect                  = user32.NewProc("ValidateRect")

	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procGetKeyState      = user32.NewProc("GetKeyState")
	procGetMessageTime   = user32.NewProc("GetMessageTime")
	procMapVirtualKeyW   = user32.NewProc("MapVirtualKeyW")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procScreenToClient   = user32.NewProc("ScreenToClient")
	procSetCapture       = user32.NewProc("SetCapture")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procSetCursor        = user32.NewProc("SetCursor")
	procTrackMouseEvent  = user32.NewProc("TrackMouseEvent")

	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
	procGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	procGlobalFree               = kernel32.NewProc("GlobalFree")
	procGlobalLock               = kernel32.NewProc("GlobalLock")
	procGlobalSize               = kernel32.NewProc("GlobalSize")
	procGlobalUnlock             = kernel32.NewProc("GlobalUnlock")

	procImmAssociateContextEx    = imm32.NewProc("ImmAssociateContextEx")
	procImmGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")
	procImmGetContext            = imm32.NewProc("ImmGetContext")
	procImmNotifyIME             = imm32.NewProc("ImmNotifyIME")
	procImmReleaseContext        = imm32.NewProc("ImmReleaseContext")
	procImmSetCandidateWindow    = imm32.NewProc("ImmSetCandidateWindow")
	procImmSetCompositionFontW   = imm32.NewProc("ImmSetCompositionFontW")
	procImmSetCompositionWindow  = imm32.NewProc("ImmSetCompositionWindow")
)

// loadProcs resolves every procedure up front, so that a missing symbol is an
// initialization error instead of a panic in the middle of a frame.
func loadProcs() error {
	for _, p := range []*windows.LazyProc{
		procAdjustWindowRectExForDpi, procCreateWindowExW, procDefWindowProcW,
		procDestroyWindow, procDispatchMessageW, procGetClientRect,
		procGetDpiForWindow, procKillTimer, procLoadCursorW,
		procMsgWaitForMultipleObjectsEx, procPeekMessageW, procPostMessageW,
		procRegisterClassExW, procSetProcessDpiAwarenessContext, procSetTimer,
		procSetWindowPos, procSetWindowTextW, procShowWindow, procTranslateMessage,
		procUnregisterClassW, procValidateRect, procGetModuleHandleW,
		procCloseClipboard, procEmptyClipboard, procGetClipboardData, procGetKeyState,
		procGetMessageTime, procMapVirtualKeyW, procOpenClipboard, procReleaseCapture, procScreenToClient,
		procSetCapture, procSetClipboardData, procSetCursor, procTrackMouseEvent,
		procGlobalAlloc, procGlobalFree, procGlobalLock, procGlobalSize, procGlobalUnlock,
		procImmAssociateContextEx, procImmGetCompositionStringW, procImmGetContext,
		procImmNotifyIME, procImmReleaseContext, procImmSetCandidateWindow,
		procImmSetCompositionFontW, procImmSetCompositionWindow,
		procWindowFromPoint, procGetAncestor, procGetUserDefaultLocaleName,
	} {
		if err := p.Find(); err != nil {
			return fmt.Errorf("win32: load %s: %w", p.Name, err)
		}
	}
	return nil
}
