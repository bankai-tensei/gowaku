package cocoa

import (
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

func TestAXRole_ObscuredIsSecureTextField(t *testing.T) {
	role, subrole := axRole(&semantics.Node{Role: semantics.RoleTextField, State: semantics.Obscured})
	if role != axRoleTextField || subrole != axSubroleSecureTextField {
		t.Errorf("got %s/%s, want %s/%s", role, subrole, axRoleTextField, axSubroleSecureTextField)
	}
}
