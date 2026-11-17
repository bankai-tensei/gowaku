package win32

import (
	"image"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// Tray icons are Shell_NotifyIcon with NOTIFYICON_VERSION_4, owned by a hidden
// window of their own. Their menu is a native popup menu: it belongs to no
// window of the application, so gowaku cannot draw it. Notifications are the
// balloons of the icon, which Windows 10 and 11 show as toasts. See
// docs/ADR/0051-tray-and-notifications.md.

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx       = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	procDestroyIcon            = user32.NewProc("DestroyIcon")
	procLoadIconW              = user32.NewProc("LoadIconW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap           = gdi32.NewProc("CreateBitmap")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

// The values of tray icons and popup menus.
const (
	NIM_ADD              = 0
	NIM_MODIFY           = 1
	NIM_DELETE           = 2
	NIM_SETVERSION       = 4
	NIF_MESSAGE          = 0x01
	NIF_ICON             = 0x02
	NIF_TIP              = 0x04
	NIF_INFO             = 0x10
	NIF_SHOWTIP          = 0x80
	NIIF_INFO            = 0x01
	NOTIFYICON_VERSION_4 = 4
	NIN_SELECT           = WM_USER + 0
	NIN_KEYSELECT        = WM_USER + 1
	NIN_BALLOONHIDE      = WM_USER + 3
	NIN_BALLOONTIMEOUT   = WM_USER + 4
	NIN_BALLOONUSERCLICK = WM_USER + 5
	WM_CONTEXTMENU       = 0x007B
	WM_NULL              = 0x0000
	WM_APP               = 0x8000
	WS_POPUP             = 0x80000000
	MF_STRING            = 0x0000
	MF_GRAYED            = 0x0001
	MF_CHECKED           = 0x0008
	MF_POPUP             = 0x0010
	MF_SEPARATOR         = 0x0800
	TPM_RIGHTBUTTON      = 0x0002
	TPM_NONOTIFY         = 0x0080
	TPM_RETURNCMD        = 0x0100
	IDI_APPLICATION      = 32512
	BI_RGB               = 0
	DIB_RGB_COLORS       = 0

	// wmTray is the callback message of the icons.
	wmTray = WM_APP + 1
	// notifyID is the ID of the icon that a notification without a tray
	// shows for a while.
	notifyID = 0xFFFF
)

// NOTIFYICONDATAW is the structure of Shell_NotifyIconW.
type NOTIFYICONDATAW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

// ICONINFO is the structure of CreateIconIndirect.
type ICONINFO struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// BITMAPINFOHEADER is the header of a device-independent bitmap.
type BITMAPINFOHEADER struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

// tray is one icon.
type tray struct {
	p    *Platform
	id   uint32
	icon uintptr
	opts platform.TrayOptions
	on   func(platform.TrayEvent)
}

// setText copies s into a fixed field of NOTIFYICONDATAW, cut to fit.
func setText(dst []uint16, s string) {
	units := utf16.Encode([]rune(s))
	n := copy(dst[:len(dst)-1], units)
	dst[n] = 0
}

// trayWindow returns the hidden window that owns the icons, made on first use.
func (p *Platform) trayWindow() (HWND, error) {
	if p.trayHWND == 0 {
		h, err := p.createNativeWindow(0, WS_POPUP, 0)
		if err != nil {
			return 0, err
		}
		p.trayHWND = h
		name, _ := syscall.UTF16PtrFromString("TaskbarCreated") //nolint:errcheck // no NUL
		r, _, _ := syscall.SyscallN(procRegisterWindowMessageW.Addr(), uintptr(unsafe.Pointer(name)))
		p.taskbarCreated = uint32(r)
	}
	return p.trayHWND, nil
}

// notify calls Shell_NotifyIconW.
func notify(msg uintptr, d *NOTIFYICONDATAW) bool {
	d.cbSize = uint32(unsafe.Sizeof(*d))
	// d is only read during the call.
	r, _, _ := syscall.SyscallN(procShellNotifyIconW.Addr(), msg, uintptr(unsafe.Pointer(d)))
	return r != 0
}

// CreateTray implements platform.Trays.
func (p *Platform) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	h, err := p.trayWindow()
	if err != nil {
		return nil, err
	}
	p.lastTray++
	t := &tray{p: p, id: p.lastTray, opts: opts, on: on, icon: iconOf(opts.Icon)}
	if !t.add(h) {
		destroyIcon(t.icon)
		return nil, platform.ErrNoTray
	}
	p.trays = append(p.trays, t)
	return t, nil
}

// add shows the icon, and again after Explorer restarts.
func (t *tray) add(h HWND) bool {
	d := NOTIFYICONDATAW{hWnd: uintptr(h), uID: t.id, uFlags: NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP,
		uCallbackMessage: wmTray, hIcon: t.icon}
	setText(d.szTip[:], t.opts.Tooltip)
	if !notify(NIM_ADD, &d) {
		return false
	}
	d.uVersion = NOTIFYICON_VERSION_4
	return notify(NIM_SETVERSION, &d)
}

// Update implements platform.Tray.
func (t *tray) Update(opts platform.TrayOptions) {
	if t.id == 0 {
		return
	}
	old := t.icon
	t.opts, t.icon = opts, iconOf(opts.Icon)
	d := NOTIFYICONDATAW{hWnd: uintptr(t.p.trayHWND), uID: t.id, uFlags: NIF_ICON | NIF_TIP | NIF_SHOWTIP, hIcon: t.icon}
	setText(d.szTip[:], opts.Tooltip)
	notify(NIM_MODIFY, &d)
	destroyIcon(old)
}

// Close implements platform.Tray.
func (t *tray) Close() {
	if t.id == 0 {
		return
	}
	d := NOTIFYICONDATAW{hWnd: uintptr(t.p.trayHWND), uID: t.id}
	notify(NIM_DELETE, &d)
	destroyIcon(t.icon)
	for i, o := range t.p.trays {
		if o == t {
			t.p.trays = append(t.p.trays[:i], t.p.trays[i+1:]...)
			break
		}
	}
	t.id = 0
}

// Notify implements platform.Trays: the balloon of the first icon, or of an
// icon shown for the time of the balloon.
func (p *Platform) Notify(title, body string) error {
	h, err := p.trayWindow()
	if err != nil {
		return err
	}
	d := NOTIFYICONDATAW{hWnd: uintptr(h), uFlags: NIF_INFO, dwInfoFlags: NIIF_INFO}
	setText(d.szInfoTitle[:], title)
	setText(d.szInfo[:], body)
	if len(p.trays) > 0 {
		d.uID = p.trays[0].id
		if !notify(NIM_MODIFY, &d) {
			return platform.ErrNoNotifications
		}
		return nil
	}
	if !p.notifyIcon {
		icon := NOTIFYICONDATAW{hWnd: uintptr(h), uID: notifyID, uFlags: NIF_ICON, hIcon: iconOf(nil)}
		if !notify(NIM_ADD, &icon) {
			return platform.ErrNoNotifications
		}
		icon.uVersion = NOTIFYICON_VERSION_4
		notify(NIM_SETVERSION, &icon)
		p.notifyIcon = true
	}
	d.uID = notifyID
	if !notify(NIM_MODIFY, &d) {
		return platform.ErrNoNotifications
	}
	return nil
}

// dropNotifyIcon takes away the icon that a notification without a tray
// showed, once its balloon is gone.
func (p *Platform) dropNotifyIcon() {
	if p.notifyIcon {
		d := NOTIFYICONDATAW{hWnd: uintptr(p.trayHWND), uID: notifyID}
		notify(NIM_DELETE, &d)
		p.notifyIcon = false
	}
}

// destroyTrays takes every icon away, when the platform goes.
func (p *Platform) destroyTrays() {
	for len(p.trays) > 0 {
		p.trays[len(p.trays)-1].Close()
	}
	p.dropNotifyIcon()
	if p.trayHWND != 0 {
		_, _, _ = syscall.SyscallN(procDestroyWindow.Addr(), uintptr(p.trayHWND))
		p.trayHWND = 0
	}
}

// trayMessage handles the messages of the hidden window of the icons.
func (p *Platform) trayMessage(msg, wParam, lParam uintptr) (uintptr, bool) {
	switch {
	case msg == wmTray:
		p.trayEvent(uint32(lParam&0xFFFF), uint32(lParam>>16&0xFFFF), wParam)
		return 0, true
	case msg == WM_TIMER && wParam == modalTimerID:
		p.runModalHandler()
		return 0, true
	case p.taskbarCreated != 0 && msg == uintptr(p.taskbarCreated):
		// Explorer restarted, and forgot every icon.
		for _, t := range p.trays {
			t.add(p.trayHWND)
		}
		p.notifyIcon = false
		return 0, true
	}
	return 0, false
}

// trayEvent turns the callback of an icon into a TrayEvent.
func (p *Platform) trayEvent(event, id uint32, anchor uintptr) {
	if id == notifyID {
		if event == NIN_BALLOONHIDE || event == NIN_BALLOONTIMEOUT || event == NIN_BALLOONUSERCLICK {
			p.dropNotifyIcon()
		}
		return
	}
	var t *tray
	for _, o := range p.trays {
		if o.id == id {
			t = o
		}
	}
	if t == nil || t.on == nil {
		return
	}
	switch event {
	case NIN_SELECT, NIN_KEYSELECT:
		t.on(platform.TrayEvent{Kind: platform.TrayActivate})
	case NIN_BALLOONUSERCLICK:
		t.on(platform.TrayEvent{Kind: platform.TrayNotificationClick})
	case WM_CONTEXTMENU:
		x, y := int32(int16(anchor&0xFFFF)), int32(int16(anchor>>16&0xFFFF))
		if item, ok := p.trackMenu(t.opts.Items, x, y); ok {
			t.on(platform.TrayEvent{Kind: platform.TrayChoose, Item: item})
		}
	}
}

// trackMenu shows items as a popup menu at (x, y) on the screen, and returns
// the ID of the chosen item. The menu runs a modal loop, in which the modal
// handler keeps the application drawing.
func (p *Platform) trackMenu(items []platform.TrayItem, x, y int32) (int, bool) {
	if len(items) == 0 {
		return 0, false
	}
	menu := buildMenu(items)
	defer func() { _, _, _ = syscall.SyscallN(procDestroyMenu.Addr(), menu) }()
	// Without the foreground, the menu would not close on a click beside it.
	_, _, _ = syscall.SyscallN(procSetForegroundWindow.Addr(), uintptr(p.trayHWND))
	p.enterModal(p.trayHWND)
	r, _, _ := syscall.SyscallN(procTrackPopupMenuEx.Addr(), menu, TPM_RETURNCMD|TPM_RIGHTBUTTON|TPM_NONOTIFY,
		uintptr(x), uintptr(y), uintptr(p.trayHWND), 0)
	p.exitModal()
	_, _, _ = syscall.SyscallN(procPostMessageW.Addr(), uintptr(p.trayHWND), WM_NULL, 0, 0)
	// Command 0 means nothing was chosen; IDs are shifted by one for it.
	if r == 0 {
		return 0, false
	}
	return int(r) - 1, true
}

// buildMenu makes a popup menu of items. The menu owns its submenus.
func buildMenu(items []platform.TrayItem) uintptr {
	menu, _, _ := syscall.SyscallN(procCreatePopupMenu.Addr())
	for i := range items {
		it := &items[i]
		if it.Separator {
			_, _, _ = syscall.SyscallN(procAppendMenuW.Addr(), menu, MF_SEPARATOR, 0, 0)
			continue
		}
		label := it.Label
		if it.Shortcut != "" {
			label += "\t" + it.Shortcut
		}
		text, err := syscall.UTF16PtrFromString(label)
		if err != nil {
			continue
		}
		flags, id := uintptr(MF_STRING), uintptr(it.ID+1)
		if !it.Enabled {
			flags |= MF_GRAYED
		}
		if it.Checkable && it.Checked {
			flags |= MF_CHECKED
		}
		if len(it.Items) > 0 {
			flags, id = flags|MF_POPUP, buildMenu(it.Items)
		}
		// text is only read during the call.
		_, _, _ = syscall.SyscallN(procAppendMenuW.Addr(), menu, flags, id, uintptr(unsafe.Pointer(text)))
	}
	return menu
}

// iconOf makes an icon of img, or returns that of applications for nil.
func iconOf(img *image.NRGBA) uintptr {
	if img == nil || img.Rect.Dx() == 0 || img.Rect.Dy() == 0 {
		r, _, _ := syscall.SyscallN(procLoadIconW.Addr(), 0, IDI_APPLICATION)
		return r
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	header := BITMAPINFOHEADER{biWidth: int32(w), biHeight: -int32(h), biPlanes: 1, biBitCount: 32, biCompression: BI_RGB}
	header.biSize = uint32(unsafe.Sizeof(header))
	var bits uintptr
	// header and bits are used during the call; the section owns its pixels.
	color, _, _ := syscall.SyscallN(procCreateDIBSection.Addr(), 0, uintptr(unsafe.Pointer(&header)), DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == 0 {
		r, _, _ := syscall.SyscallN(procLoadIconW.Addr(), 0, IDI_APPLICATION)
		return r
	}
	defer func() { _, _, _ = syscall.SyscallN(procDeleteObject.Addr(), color) }()
	// Reading through &bits turns the integer into a pointer without a
	// uintptr-to-pointer conversion. The pixels live until DeleteObject.
	px := unsafe.Slice(*(**byte)(unsafe.Pointer(&bits)), 4*w*h)
	for y := range h {
		src := img.Pix[y*img.Stride:]
		for x := range w {
			s, d := src[4*x:], px[4*(y*w+x):]
			d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
		}
	}
	mask, _, _ := syscall.SyscallN(procCreateBitmap.Addr(), uintptr(w), uintptr(h), 1, 1, 0)
	defer func() { _, _, _ = syscall.SyscallN(procDeleteObject.Addr(), mask) }()
	info := ICONINFO{fIcon: 1, hbmMask: mask, hbmColor: color}
	// info is only read during the call; the icon copies the bitmaps.
	icon, _, _ := syscall.SyscallN(procCreateIconIndirect.Addr(), uintptr(unsafe.Pointer(&info)))
	return icon
}

// destroyIcon frees an icon made by iconOf. The shared icon of applications
// must not be freed, and DestroyIcon refuses it anyway.
func destroyIcon(icon uintptr) {
	if icon != 0 {
		_, _, _ = syscall.SyscallN(procDestroyIcon.Addr(), icon)
	}
}
