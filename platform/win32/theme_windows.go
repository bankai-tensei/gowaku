package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/bankai-tensei/gowaku/platform"
)

const (
	// personalizeKey holds the app mode of the user. It is not documented as an
	// API, but every toolkit reads it, and there is no other for Win32 programs.
	personalizeKey   = `Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize`
	appsUseLightName = "AppsUseLightTheme"
	// immersiveColorSet is the area named by WM_SETTINGCHANGE for the app mode.
	immersiveColorSet = "ImmersiveColorSet"
	// intlArea is the area named by WM_SETTINGCHANGE when the user changes the
	// regional format.
	intlArea = "intl"
	// settingAreaLimit bounds the walk over the string of WM_SETTINGCHANGE.
	settingAreaLimit = 64

	// SPI_GETCLIENTAREAANIMATION reads "Animation effects" of the accessibility
	// settings.
	SPI_GETCLIENTAREAANIMATION = 0x1042
	// SPI_SETCLIENTAREAANIMATION is what WM_SETTINGCHANGE carries when the
	// setting changes.
	SPI_SETCLIENTAREAANIMATION = 0x1043
)

// ReduceMotion implements platform.Platform. It is true when the user turned
// "Animation effects" off.
func (p *Platform) ReduceMotion() bool {
	var on int32 = 1
	r, _, _ := procSystemParametersInfoW.Call(SPI_GETCLIENTAREAANIMATION, 0, uintptr(unsafe.Pointer(&on)), 0)
	return r != 0 && on == 0
}

// Brightness implements platform.Platform.
func (p *Platform) Brightness() platform.Brightness { return readBrightness() }

func readBrightness() platform.Brightness {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return platform.BrightnessLight
	}
	defer func() {
		_ = k.Close() //nolint:errcheck // nothing can be done about it
	}()
	v, _, err := k.GetIntegerValue(appsUseLightName)
	if err == nil && v == 0 {
		return platform.BrightnessDark
	}
	return platform.BrightnessLight
}

// settingChange reports a change of the app mode, of the animation setting or of
// the regional format. Every top-level window gets the message; the application
// compares what it reads, so that repeats cost nothing.
func (p *Platform) settingChange(wParam, lParam uintptr) {
	if wParam == SPI_SETCLIENTAREAANIMATION {
		p.queue.Push(platform.Event{Kind: platform.EventSettings})
		return
	}
	if lParam == 0 {
		return
	}
	// lParam points at a NUL-terminated UTF-16 string that the sender keeps alive
	// for the duration of the message.
	area := windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&lParam)))
	switch {
	case len(area) >= settingAreaLimit:
	case area == immersiveColorSet:
		p.queue.Push(platform.Event{Kind: platform.EventBrightness})
	case area == intlArea:
		// The application compares the locale, as it does the brightness.
		p.queue.Push(platform.Event{Kind: platform.EventSettings})
	}
}

// SetDarkTitleBar implements platform.Window.
func (w *Window) SetDarkTitleBar(dark bool) {
	var value int32
	if dark {
		value = 1
	}
	// On a Windows too old for the attribute the title bar stays light, which is
	// the best that can happen there.
	_, _, _ = procDwmSetWindowAttribute.Call(uintptr(w.hwnd), DWMWA_USE_IMMERSIVE_DARK_MODE,
		uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
