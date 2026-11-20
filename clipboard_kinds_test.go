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
