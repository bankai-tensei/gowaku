//go:build gpu

package win32

import (
	"image"
	"image/color"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// These put an icon in the tray of the user for a moment and open its menu, so
// they run with the GPU tests, locally only. They show no notification, which
// would stay in the notification center.

var procFindWindowExW = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowExW")

func TestTray_IconActivateAndMenu(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	icon := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range 16 {
		icon.SetNRGBA(i, i, color.NRGBA{0x2f, 0x5f, 0xd0, 255})
	}
	var events []platform.TrayEvent
	tr, err := p.CreateTray(platform.TrayOptions{Icon: icon, Tooltip: "gowaku test", Items: []platform.TrayItem{
		{ID: 7, Label: "開く(&O)", Enabled: true},
		{Separator: true},
		{ID: 8, Label: "表示", Enabled: true, Items: []platform.TrayItem{{ID: 9, Label: "手前", Enabled: true, Checkable: true, Checked: true}}},
	}}, func(e platform.TrayEvent) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	id := tr.(*tray).id

	// A click on the icon, as NOTIFYICON_VERSION_4 reports it.
	p.trayEvent(NIN_SELECT, id, 0)
	if len(events) != 1 || events[0].Kind != platform.TrayActivate {
		t.Fatalf("events %+v", events)
	}

	// The menu, chosen from its own keys: Down to the first item, Enter.
	menuClass, _ := windows.UTF16PtrFromString("#32768") //nolint:errcheck // no NUL
	go func() {
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			m, _, _ := procFindWindowExW.Call(0, 0, uintptr(unsafe.Pointer(menuClass)), 0)
			if m != 0 {
				procPostMessageW.Call(m, WM_KEYDOWN, 0x28, 0) // VK_DOWN
				procPostMessageW.Call(m, WM_KEYDOWN, 0x0D, 0) // VK_RETURN
				return
			}
		}
	}()
	p.trayEvent(WM_CONTEXTMENU, id, 100|100<<16)
	if len(events) != 2 || events[1].Kind != platform.TrayChoose || events[1].Item != 7 {
		t.Errorf("events after the menu %+v", events)
	}
	tr.Update(platform.TrayOptions{Tooltip: "updated"})
}
