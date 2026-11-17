package win32

import (
	"testing"
	"unsafe"
)

// The expected values were printed by a C program built against the
// mingw-w64 headers, not derived from the Go definitions.
func TestABI_StructLayout(t *testing.T) {
	tests := []struct {
		name      string
		got, want uintptr
	}{
		{"sizeof POINT", unsafe.Sizeof(POINT{}), 8},
		{"sizeof RECT", unsafe.Sizeof(RECT{}), 16},
		{"sizeof MONITORINFO", unsafe.Sizeof(MONITORINFO{}), 40},
		{"sizeof WINDOWPLACEMENT", unsafe.Sizeof(WINDOWPLACEMENT{}), 44},
		{"offsetof WINDOWPLACEMENT.rcNormalPosition", unsafe.Offsetof(WINDOWPLACEMENT{}.rcNormalPosition), 28},
		{"sizeof MINMAXINFO", unsafe.Sizeof(MINMAXINFO{}), 40},
		{"offsetof MINMAXINFO.ptMinTrackSize", unsafe.Offsetof(MINMAXINFO{}.ptMinTrackSize), 24},
		{"sizeof FORMATETC", unsafe.Sizeof(FORMATETC{}), 32},
		{"sizeof NOTIFYICONDATAW", unsafe.Sizeof(NOTIFYICONDATAW{}), 976},
		{"sizeof POINTER_INFO", unsafe.Sizeof(POINTER_INFO{}), 96},
		{"sizeof POINTER_PEN_INFO", unsafe.Sizeof(POINTER_PEN_INFO{}), 120},
		{"offsetof POINTER_PEN_INFO.pressure", unsafe.Offsetof(POINTER_PEN_INFO{}.pressure), 104},
		{"sizeof ICONINFO", unsafe.Sizeof(ICONINFO{}), 32},
		{"sizeof BITMAPINFOHEADER", unsafe.Sizeof(BITMAPINFOHEADER{}), 40},
		{"sizeof STGMEDIUM", unsafe.Sizeof(STGMEDIUM{}), 24},
		{"sizeof MSG", unsafe.Sizeof(MSG{}), 48},
		{"offsetof MSG.lParam", unsafe.Offsetof(MSG{}.lParam), 24},
		{"offsetof MSG.pt", unsafe.Offsetof(MSG{}.pt), 36},
		{"sizeof WNDCLASSEXW", unsafe.Sizeof(WNDCLASSEXW{}), 80},
		{"offsetof WNDCLASSEXW.lpfnWndProc", unsafe.Offsetof(WNDCLASSEXW{}.lpfnWndProc), 8},
		{"offsetof WNDCLASSEXW.hIconSm", unsafe.Offsetof(WNDCLASSEXW{}.hIconSm), 72},
		{"sizeof TRACKMOUSEEVENT", unsafe.Sizeof(TRACKMOUSEEVENT{}), 24},
		{"offsetof TRACKMOUSEEVENT.dwHoverTime", unsafe.Offsetof(TRACKMOUSEEVENT{}.dwHoverTime), 16},
		{"sizeof COMPOSITIONFORM", unsafe.Sizeof(COMPOSITIONFORM{}), 28},
		{"offsetof COMPOSITIONFORM.rcArea", unsafe.Offsetof(COMPOSITIONFORM{}.rcArea), 12},
		{"sizeof LOGFONTW", unsafe.Sizeof(LOGFONTW{}), 92},
		{"offsetof LOGFONTW.lfFaceName", unsafe.Offsetof(LOGFONTW{}.lfFaceName), 28},
		{"sizeof CANDIDATEFORM", unsafe.Sizeof(CANDIDATEFORM{}), 32},
		{"offsetof CANDIDATEFORM.rcArea", unsafe.Offsetof(CANDIDATEFORM{}.rcArea), 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %d, want %d", tt.got, tt.want)
			}
		})
	}
}
