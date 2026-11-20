package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

func TestClipboardKinds_MatchThePlatform(t *testing.T) {
	pairs := []struct {
		ours   ClipboardKinds
		theirs platform.ClipboardKinds
	}{
		{ClipboardText, platform.ClipboardText},
		{ClipboardHTML, platform.ClipboardHTML},
		{ClipboardImage, platform.ClipboardImage},
		{ClipboardFiles, platform.ClipboardFiles},
	}
	for _, p := range pairs {
		if uint8(p.ours) != uint8(p.theirs) {
			t.Errorf("kind %d is %d on the platform", p.ours, p.theirs)
		}
	}
}

func TestFileDialog_CrossesToThePlatform(t *testing.T) {
	d := FileDialog{Title: "t", Save: true, Name: "n", Multiple: true, Directory: true, Folder: "f", Filters: []FileFilter{{Name: "x", Patterns: []string{"*.x"}}}}
	p := platformDialog(d)
	if p.Title != "t" || !p.Save || p.Name != "n" || !p.Multiple || !p.Directory || p.Folder != "f" || len(p.Filters) != 1 || p.Filters[0].Patterns[0] != "*.x" {
		t.Errorf("got %+v", p)
	}
}
