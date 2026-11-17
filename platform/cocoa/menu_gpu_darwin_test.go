//go:build gpu && darwin && arm64

package cocoa

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

func TestPlatform_MenuBar(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "menu", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()
		chosen := -1
		w.SetMenuBar([]platform.MenuItem{{Label: "ファイル(&F)", Items: []platform.MenuItem{
			{ID: 0, Label: "保存(&S)", Key: input.KeyS, Modifiers: input.ModControl, Enabled: true},
			{Separator: true},
			{ID: 1, Label: "閉じる", Enabled: false},
		}}}, func(id int) { chosen = id })
		p.installMenu(w) // the window may not be key in a test run
		p.libs.pool(func() {
			bar := p.app.Send(sel("mainMenu"))
			// The application menu, ファイル, Window.
			if n := objc.Send[int64](bar, sel("numberOfItems")); n != 3 {
				t.Errorf("menu bar has %d menus, want 3", n)
				return
			}
			file := bar.Send(sel("itemAtIndex:"), int64(1)).Send(sel("submenu"))
			if got := goString(file.Send(sel("title"))); got != "ファイル" {
				t.Errorf("title %q", got)
			}
			save := file.Send(sel("itemAtIndex:"), int64(0))
			if got := goString(save.Send(sel("keyEquivalent"))); got != "s" {
				t.Errorf("key equivalent %q", got)
			}
			if objc.Send[bool](file.Send(sel("itemAtIndex:"), int64(2)), sel("isEnabled")) {
				t.Error("disabled item is enabled")
			}
			file.Send(sel("performActionForItemAtIndex:"), int64(0))
		})
		if chosen != 0 {
			t.Errorf("chosen %d, want 0", chosen)
		}
	})
}
