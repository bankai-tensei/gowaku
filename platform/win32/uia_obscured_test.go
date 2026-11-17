package win32

import (
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

func TestNodeProperty_ObscuredIsPassword(t *testing.T) {
	n := &semantics.Node{Role: semantics.RoleTextField, State: semantics.Obscured, Value: "•••"}
	if got := nodeProperty(n, UIA_IsPasswordPropertyId, false); got != propFlag(true) {
		t.Errorf("an obscured field must be a password: got %+v", got)
	}
	if got := nodeProperty(n, UIA_ControlTypePropertyId, false); got != propInt(UIA_EditControlTypeId) {
		t.Errorf("it stays an edit: got %+v", got)
	}
}
