package markdown

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// copiedFor is how long the Copy button of a code block says Copied.
const copiedFor = 1500 * time.Millisecond

// copyButton is the Copy button of a code block: after a press it shows a
// check and the word done for copiedFor, so that the reader sees the copy
// took, and a screen reader hears the new name.
type copyButton struct {
	core.StatefulBase
	label, done string
	onPress     func()
}

// CreateState implements core.StatefulWidget.
func (copyButton) CreateState() core.State { return &copyButtonState{} }

type copyButtonState struct {
	core.StateOf[copyButton]
	copied bool
	cancel func()
}

func (s *copyButtonState) press() {
	s.Widget().onPress()
	if s.cancel != nil {
		s.cancel()
	}
	s.SetState(func() { s.copied = true })
	s.cancel = s.After(copiedFor, func() {
		s.cancel = nil
		s.SetState(func() { s.copied = false })
	})
}

// Build implements core.State.
func (s *copyButtonState) Build(core.BuildContext) core.Widget {
	w := s.Widget()
	label, icon := w.label, icons.Copy()
	if s.copied && w.done != "" {
		label, icon = w.done, icons.Check()
	}
	return control.Button{Label: label, Icon: icon, Style: control.ButtonQuiet, Size: control.ButtonSmall, OnPress: s.press}
}
