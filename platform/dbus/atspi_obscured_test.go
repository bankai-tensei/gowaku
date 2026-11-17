package dbus

import (
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

func TestAtspiRole_ObscuredIsPasswordText(t *testing.T) {
	for _, st := range []semantics.State{semantics.Obscured, semantics.Obscured | semantics.Multiline} {
		got := atspiRole(&semantics.Node{Role: semantics.RoleTextField, State: st})
		if got != atspiRolePasswordText || atspiRoleName(got) != "password text" {
			t.Errorf("state %b: got %d (%s), want password text", st, got, atspiRoleName(got))
		}
	}
}
