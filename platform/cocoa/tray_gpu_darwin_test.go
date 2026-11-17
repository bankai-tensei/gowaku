//go:build gpu && darwin && arm64

package cocoa

import (
	"errors"
	"image"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

func TestPlatform_Tray(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		var got []platform.TrayEvent
		icon := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		pt, err := p.CreateTray(platform.TrayOptions{Icon: icon, Tooltip: "gowaku", Items: []platform.TrayItem{
			{ID: 7, Label: "開く(&O)", Enabled: true},
			{Separator: true},
			{ID: 9, Label: "終了", Enabled: true},
		}}, func(e platform.TrayEvent) { got = append(got, e) })
		if err != nil {
			t.Errorf("CreateTray: %v", err)
			return
		}
		tr := pt.(*tray)
		p.libs.pool(func() {
			menu := tr.item.Send(sel("menu"))
			if n := objc.Send[int64](menu, sel("numberOfItems")); n != 3 {
				t.Errorf("menu has %d items, want 3", n)
				return
			}
			if title := goString(menu.Send(sel("itemAtIndex:"), int64(0)).Send(sel("title"))); title != "開く" {
				t.Errorf("title %q", title)
			}
			menu.Send(sel("performActionForItemAtIndex:"), int64(2))
		})
		if len(got) != 1 || got[0].Kind != platform.TrayChoose || got[0].Item != 9 {
			t.Errorf("events %+v", got)
		}
		// A test binary has no bundle, so there is no notification center.
		if err := p.Notify("title", "body"); !errors.Is(err, platform.ErrNoNotifications) {
			t.Errorf("Notify: %v", err)
		}
		pt.Close()
	})
}
