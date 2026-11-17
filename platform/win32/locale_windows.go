package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// LOCALE_NAME_MAX_LENGTH is the size of a locale name with its NUL, in UTF-16
// units.
const LOCALE_NAME_MAX_LENGTH = 85

// Locale implements platform.Platform: the locale of the regional format of the
// user, from GetUserDefaultLocaleName. WM_SETTINGCHANGE with "intl" reports
// changes.
func (p *Platform) Locale() string {
	var buf [LOCALE_NAME_MAX_LENGTH]uint16
	// The buffer lives on the stack for the duration of the call.
	n, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return localeTag(windows.UTF16ToString(buf[:n]))
}
