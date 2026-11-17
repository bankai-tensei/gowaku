package layout

import "github.com/bankai-tensei/gowaku/geom"

// The render objects below only place, size or paint around their children.
// They implement core.SelfHitTester and are hit only where a child is hit, as
// in Flutter: a full-size layer of a Stack made of an Align, a Column or a
// Padding with a small child takes the pointer over that child, and lets it
// through everywhere else. What draws or takes the pointer itself stays hit
// over its whole box: a RenderDecoratedBox with a decoration that paints, the
// leaves (text, images, icons, painted controls), and the targets of the
// pointer (RenderListener, RenderGesture, RenderMouseRegion, RenderBarrier,
// RenderViewport).

// HitTestSelf implements core.SelfHitTester. RenderAlign is hit only over its child.
func (*RenderAlign) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderPadding is hit only over its child.
func (*RenderPadding) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderConstrainedBox is hit only over its child.
func (*RenderConstrainedBox) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderAspectRatio is hit only over its child.
func (*RenderAspectRatio) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderFlex is hit only over its children.
func (*RenderFlex) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderFlexible is hit only over its child.
func (*RenderFlexible) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderStack is hit only over its children.
func (*RenderStack) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderPositioned is hit only over its child.
func (*RenderPositioned) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderPositionedFill is hit only over its child.
func (*RenderPositionedFill) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderWrap is hit only over its children.
func (*RenderWrap) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderGrid is hit only over its children.
func (*RenderGrid) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderCustomLayout is hit only over its children.
func (*RenderCustomLayout) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderClip is hit only over its child.
func (*RenderClip) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderTranslate is hit only over its child.
func (*RenderTranslate) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderOpacity is hit only over its child.
func (*RenderOpacity) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderScale is hit only over its child.
func (*RenderScale) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderShaderMask is hit only over its child.
func (*RenderShaderMask) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderRing is hit only over its child.
func (*RenderRing) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderSemantics is hit only over its child.
func (*RenderSemantics) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderExcludeSemantics is hit only over its child.
func (*RenderExcludeSemantics) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderFocus is hit only over its child.
func (*RenderFocus) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderFocusOrder is hit only over its child.
func (*RenderFocusOrder) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderShortcuts is hit only over its child.
func (*RenderShortcuts) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. RenderModal is hit only over its
// child; the barrier under a dialog is what takes the presses beside it.
func (*RenderModal) HitTestSelf(geom.Offset) bool { return false }

// HitTestSelf implements core.SelfHitTester. A RenderDecoratedBox is hit over
// its whole box when its decoration paints (a color, a gradient, a border or a
// shadow), as a card or a filled Container is; one that paints nothing is hit
// only over its child.
func (r *RenderDecoratedBox) HitTestSelf(geom.Offset) bool {
	d := &r.Decoration
	return d.Color.A > 0 || d.Gradient != nil || d.Border != nil || len(d.Shadows) > 0
}
