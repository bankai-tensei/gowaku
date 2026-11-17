// Package gowakutest runs a widget tree in a test, without a window or a GPU, the
// way gowaku.Run would run it in a window. A test finds widgets by what they
// say to assistive technology, taps and types like a user, moves a fake clock,
// and takes screenshots drawn on the CPU.
//
//	tt := gowakutest.New(t, counter{}, gowakutest.Options{})
//	tt.Tap("Add one")
//	if _, ok := tt.Find("count: 1"); !ok {
//		t.Error("the count did not change")
//	}
//
// The methods that act like a user (the taps Tap, SecondaryTap, MiddleTap
// and DoubleTap, Hover, Scroll, Drag, DropFiles and DropText, their At forms,
// Unhover, Type and Key) settle after they act. Those that take a label fail
// the test when a user could not reach the node: outside the window, clipped
// away or covered; see Tap. Press, MoveTo and Release hold
// a drag in progress, to look at it on the way. Hold keeps modifier keys down
// for them, for a Shift-click or a Ctrl-drag. The Tester has a clipboard of
// its own, which a copy fills and a paste reads: Clipboard and SetClipboard.
// It records what gowaku.ShowTray and gowaku.Notify show, in place of the
// desktop: Trays, whose icons a test clicks, and Notifications.
//
// Time stands still unless the test moves it, so animations, periodic tickers
// and the delays of core.StateBase.After run exactly as far as the test says.
// The rule is this:
//
//   - Settle draws frames, FrameTime apart, only while something animates, and
//     stops as soon as nothing does. It never jumps the clock ahead to a
//     sleeping ticker: a periodic ticker, the delay of an After or that of a
//     tooltip. Such a wake runs during Settle only if it comes due within the
//     frames of an animation, such as the press of a button that a tap
//     animates, so a test must not count on it.
//   - Advance(d) moves the clock by d and runs every wake that comes due on
//     the way, at its time. After(16ms) from a tap therefore needs
//     Advance(16 * time.Millisecond), or more, after the tap.
//
// A tree that never comes to rest, such as one that spins a picture with a
// ticker on every frame, sets Options.Settle, so that Settle, and with it the
// methods that act, advance the clock by that fixed time instead, wakes
// included. Post from gowaku drops its function outside Run; drive a tree with
// tickers or After instead, or call the code that Post would have run.
//
// A Tester belongs to the goroutine of its test.
package gowakutest

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	defaultWidth  = 800
	defaultHeight = 600
	// clickGap is the time on the pointer clock between two taps, longer than
	// a double click takes, and pressTime that between a press and its
	// release, or a release and the next press of a double click.
	clickGap  = 2000
	pressTime = 40
	// FrameTime is the time between the frames that Advance and Settle draw
	// while something animates, as on a 60 Hz display.
	FrameTime = 16 * time.Millisecond
	// maxSettle bounds how long Settle waits for the tree to come to rest.
	maxSettle = time.Minute
)

// Options configures New.
type Options struct {
	// Width and Height are the window in logical pixels. Zero selects 800 by
	// 600.
	Width, Height float32
	// Scale is the number of physical pixels per logical one in screenshots.
	// Zero selects 1.
	Scale float32
	// Theme is the look, as in gowaku.App. Dark picks its dark half; the light
	// half is the default, so that results do not depend on the OS.
	Theme theme.Set
	Dark  bool
	// Fonts are the fonts to set text in. Nil uses the fonts of the system,
	// which Japanese text needs; give fixed fonts for screenshots that must
	// match on every machine.
	Fonts [][]byte
	// ReduceMotion stands for the OS setting that asks for less animation.
	ReduceMotion bool
	// Locale is the language and region of the window, as App.Locale in
	// gowaku: the standard widgets take their words, the names of months and
	// days and the first day of the week from it, through basic.MediaQueryData
	// and basic.Localizations. The zero value is English, whatever the locale
	// of the OS, so that results do not depend on the machine.
	Locale language.Tag
	// Strings are the tables of words the application adds, as App.Strings.
	Strings []*basic.Strings
	// Settle is how far the taps, Type, Key and Settle move the clock
	// after they act, drawing the frames on the way as Advance does, for a
	// tree that never comes to rest: an orbit that a ticker turns on every
	// frame, a spinner that stays. Zero, or less, waits until nothing animates
	// any more, which fails the test for such a tree. 500 milliseconds lets the
	// usual transitions of the controls and the pages end.
	Settle time.Duration
}

// Node is an entry of the accessibility tree.
type Node = semantics.Node

// Tester holds a tree and its fake clock.
type Tester struct {
	t      testing.TB
	tree   *core.Tree
	shaper *text.Shaper
	theme  *theme.Theme
	size   geom.Size
	scale  float32
	now    time.Duration
	settle time.Duration
	// pointerTime is the millisecond clock of the pointer events of the
	// taps. See click.
	pointerTime uint32
	// held is what Hold keeps down.
	held input.Modifiers
	// pressed says that PressAt holds the primary button down, at pointer.
	pressed bool
	pointer geom.Offset
	// host keeps the clipboard, the trays and the notifications.
	host *host

	device    *soft.Device
	resources *render.Resources
	canvas    paint.Canvas
	batch     gpu.Batch
	// posted is signaled when a State's Poster queues work. See WaitUntil.
	posted chan struct{}
}

// New mounts root and lays it out. It fails the test if the fonts cannot be
// loaded, and disposes of the tree when the test ends.
func New(t testing.TB, root core.Widget, opts Options) *Tester {
	t.Helper()
	if root == nil {
		t.Fatal("gowakutest: New needs a root widget")
	}
	var shaper *text.Shaper
	var err error
	if opts.Fonts == nil {
		shaper, err = text.NewSystemShaper()
	} else {
		shaper, err = text.NewShaper(opts.Fonts...)
	}
	if err != nil {
		t.Fatalf("gowakutest: load fonts: %v", err)
	}
	b := theme.BrightnessLight
	if opts.Dark {
		b = theme.BrightnessDark
	}
	tt := &Tester{
		t:      t,
		shaper: shaper,
		theme:  opts.Theme.For(b),
		size:   geom.Size{W: opts.Width, H: opts.Height},
		scale:  opts.Scale,
		settle: max(opts.Settle, 0),
		host:   &host{},
	}
	tt.host.tt = tt
	if tt.size.W <= 0 {
		tt.size.W = defaultWidth
	}
	if tt.size.H <= 0 {
		tt.size.H = defaultHeight
	}
	if tt.scale <= 0 {
		tt.scale = 1
	}
	// The same services as the root of a window of Run.
	tt.tree = core.NewTree(core.Inherited[*text.Shaper]{Value: shaper, Child: basic.MediaQuery{
		Data: basic.MediaQueryData{Brightness: b, Scale: tt.scale, ReduceMotion: opts.ReduceMotion, Locale: opts.Locale, Size: tt.size},
		Child: basic.Localizations{Strings: opts.Strings, Child: basic.Theme{
			Data: tt.theme, Child: overlay.Overlay{Child: root},
		}},
	}})
	tt.tree.SetHost(tt.host)
	tt.posted = make(chan struct{}, 1)
	tt.tree.SetWake(func() {
		select {
		case tt.posted <- struct{}{}:
		default:
		}
	})
	// A window under test has the keyboard, as the front window would.
	tt.tree.SetWindowFocused(true)
	t.Cleanup(tt.close)
	tt.Pump()
	return tt
}

func (tt *Tester) close() {
	tt.tree.Dispose()
	if tt.resources != nil {
		tt.resources.Destroy()
		tt.device.Destroy()
	}
}

// Hold keeps the modifier keys mods down for what the Tester does next, until
// the next Hold: Hold(0) lets them go. The taps, Hover, Drag and the drops
// carry them, so that Hold(input.ModShift) then Tap(label) is a Shift-click and
// Hold(input.ModControl) then DragAt(from, to) a Ctrl-drag; Scroll and Key add
// them to the modifiers they are given. Hold sends no key presses of its own.
func (tt *Tester) Hold(mods input.Modifiers) { tt.held = mods }

// Tree returns the tree, for what the Tester does not offer.
func (tt *Tester) Tree() *core.Tree { return tt.tree }

// Now returns the time of the fake clock, which starts at zero. Tickers get
// this time, so the first tick of a test happens at 0; a State that marks
// "not set" with a zero or negative time breaks here, and with the real clock
// as well, which starts at the launch of the App.
func (tt *Tester) Now() time.Duration { return tt.now }

// Pump runs one frame at the current time: the tickers that are due, build,
// layout and paint into a display list. The methods that act call it on their
// own.
func (tt *Tester) Pump() {
	tt.tree.Tick(tt.now)
	tt.frame()
}

func (tt *Tester) frame() {
	c := geom.Tight(tt.size)
	tt.tree.Build()
	tt.tree.Layout(c)
	for tt.tree.NeedsBuild() {
		tt.tree.Build()
		tt.tree.Layout(c)
	}
	tt.tree.Repaint(&tt.canvas)
}

// Advance moves the clock forward by d and draws the frames that a window
// would draw on the way: one every FrameTime while something animates, and
// one at each wake of a sleeping ticker, such as a periodic one. Advance(0),
// or less, draws one frame at the current time, as Pump does.
func (tt *Tester) Advance(d time.Duration) {
	if d <= 0 {
		tt.Pump()
		return
	}
	end := tt.now + d
	for tt.now < end {
		next := end
		if tt.tree.Ticking() || tt.tree.NeedsFrame() {
			next = min(tt.now+FrameTime, end)
		} else if wake, ok := tt.tree.NextWake(); ok && wake < end {
			next = max(wake, tt.now)
		}
		if next == tt.now {
			// A wake that is already due.
			next = min(tt.now+FrameTime, end)
		}
		tt.now = next
		tt.Pump()
	}
}

// Settle draws frames, FrameTime apart, until nothing animates any more.
// Sleeping tickers, such as periodic ones, the delays of core.StateBase.After
// and that of a tooltip, do not keep it going, and it does not move the clock
// ahead to them: a wake runs only if it comes due within the frames of an
// animation. Advance moves the clock to them. Settle fails the test if the
// tree is still moving after a minute of fake time. With Options.Settle it
// advances the clock by that much instead, running the wakes on the way, and
// never fails.
func (tt *Tester) Settle() {
	tt.t.Helper()
	if tt.settle > 0 {
		tt.Advance(tt.settle)
		return
	}
	start := tt.now
	for tt.tree.Ticking() || tt.tree.NeedsFrame() {
		if tt.now-start > maxSettle {
			tt.t.Fatal("gowakutest: the tree never comes to rest; " +
				"for one that animates for ever, set Options.Settle")
		}
		tt.now += FrameTime
		tt.Pump()
	}
}

// Semantics returns the accessibility tree. A change since the last frame,
// such as that of an event the test dispatched to Tree on its own, is built and
// laid out first, without moving the clock, so that the tree says what the
// next frame would show.
func (tt *Tester) Semantics() *Node {
	if tt.tree.NeedsFrame() {
		tt.frame()
	}
	return tt.tree.Semantics()
}

// Dump returns the accessibility tree as indented text, one node per line, for
// the message of a failing test. Each line ends with the Rect of the node, in
// logical pixels of the window. The rows that a list builds just outside its
// view are there too, with places outside the list.
func (tt *Tester) Dump() string {
	var b strings.Builder
	dump(&b, tt.Semantics(), 0)
	return b.String()
}

func dump(b *strings.Builder, n *Node, depth int) {
	if n.Role != semantics.RoleNone || n.Label != "" || n.Value != "" {
		fmt.Fprintf(b, "%s%v %q", strings.Repeat("  ", depth), n.Role, n.Label)
		if n.Value != "" {
			fmt.Fprintf(b, " = %q", n.Value)
		}
		fmt.Fprintf(b, " at %g,%g %gx%g\n", n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H)
		depth++
	}
	for _, c := range n.Children {
		dump(b, c, depth)
	}
}

// Find returns the node whose label is label: a button, a field or a slider by
// its Label, a text by its content. When several share the label, a control
// that can act wins: one with an action such as a tap, a focus or a value to
// set, or a text field. Then any other control, such as a heading or a group,
// then a text, and among equals the first in reading order. So the caption
// that field.Input, field.DropdownMenu or field.NumberField draws above
// itself with the same Label finds the field, and Tap(label) taps the field,
// not its caption.
//
// The label must match whole, as assistive technology reads it, and Dump
// shows those labels. A text inside a control that says the same as the
// control is not in the tree, so Find returns the control. A button without a
// Label, such as a pressable Card, is named by all of its texts joined with
// spaces, and those texts are not in the tree on their own. What nobody can
// see is not in the tree either: an Opacity of 0, as at the start of a fade
// in, the pages under the top of a Navigator, a collapsed pane. Nor is what
// lies under an open dialog or sheet (overlay.ShowDialog, overlay.ShowSheet),
// as a screen reader does not reach it, so that a label the page and the
// dialog share finds the dialog's; the page is back once the dialog closes.
//
// The Rect of the node is in logical pixels of the window, as in Dump. A list
// builds a few rows just outside its view, so a node may be found whose Rect
// lies outside the list, or the window: scroll it into view before a tap,
// which fails the test otherwise.
//
// The gap that Japanese typesetting puts between kana or kanji and Latin
// letters or digits is spacing, not a character: "いぶき 1号" on screen is
// labeled "いぶき1号" unless its text has a space.
func (tt *Tester) Find(label string) (*Node, bool) {
	return best(tt.FindAll(label))
}

// best returns the node of all that Find prefers.
func best(all []*Node) (*Node, bool) {
	var b *Node
	for _, n := range all {
		if b == nil || rank(n) < rank(b) {
			b = n
		}
	}
	return b, b != nil
}

// acting are the actions that make a node one that Find prefers.
const acting = semantics.ActionTap | semantics.ActionFocus | semantics.ActionSetValue |
	semantics.ActionIncrease | semantics.ActionDecrease | semantics.ActionExpand | semantics.ActionCollapse

// rank orders the nodes that share a label for Find, lowest first: a control
// that can act, another control, a text that can act, such as one that takes
// the focus to be selected, and a plain text or a node without a role.
func rank(n *Node) int {
	r := 0
	if n.Role == semantics.RoleText || n.Role == semantics.RoleNone {
		r = 2
	}
	if n.Actions&acting == 0 && n.State&semantics.Focusable == 0 && n.Role != semantics.RoleTextField {
		r++
	}
	return r
}

// FindAll returns every node whose label is label, in reading order. Like
// Find, it sees only the dialog while one is open.
func (tt *Tester) FindAll(label string) []*Node { return findAll(tt.Semantics(), label, -1, nil) }

// FindRole returns the first node, in reading order, of role whose label is
// label, matched whole as Find matches it. It tells apart nodes that share a
// label, such as a tab and the heading of its page, or a column header and a
// button:
//
//	n, ok := tt.FindRole(semantics.RoleTab, "Settings")
//	if ok {
//		tt.TapAt(n.Rect.Center())
//	}
func (tt *Tester) FindRole(role semantics.Role, label string) (*Node, bool) {
	all := tt.FindAllRole(role, label)
	if len(all) == 0 {
		return nil, false
	}
	return all[0], true
}

// FindAllRole returns every node of role whose label is label, in reading
// order. Like Find, it sees only the dialog while one is open.
func (tt *Tester) FindAllRole(role semantics.Role, label string) []*Node {
	return findAll(tt.Semantics(), label, int(role), nil)
}

// FindFunc returns the first node, in reading order, for which match reports
// true, for what a label does not pick out: any button, a node of a role with
// whatever label, a field that holds a value.
//
//	n, ok := tt.FindFunc(func(n *gowakutest.Node) bool { return n.Role == semantics.RoleSlider })
//
// Like Find, it sees only the dialog while one is open.
func (tt *Tester) FindFunc(match func(*Node) bool) (*Node, bool) {
	all := findFunc(tt.Semantics(), match, nil)
	if len(all) == 0 {
		return nil, false
	}
	return all[0], true
}

// FindAllFunc returns every node for which match reports true, in reading
// order: every row of a list, every tab, whatever their labels. Like Find, it
// sees only the dialog while one is open.
func (tt *Tester) FindAllFunc(match func(*Node) bool) []*Node {
	return findFunc(tt.Semantics(), match, nil)
}

// findFunc appends the nodes at and below n that match to out.
func findFunc(n *Node, match func(*Node) bool, out []*Node) []*Node {
	if match(n) {
		out = append(out, n)
	}
	for _, c := range n.Children {
		out = findFunc(c, match, out)
	}
	return out
}

// findAll appends the nodes at and below n labeled label, of role if it is
// not negative, to out.
func findAll(n *Node, label string, role int, out []*Node) []*Node {
	if n.Label == label && (role < 0 || int(n.Role) == role) {
		out = append(out, n)
	}
	for _, c := range n.Children {
		out = findAll(c, label, role, out)
	}
	return out
}

// Tap presses and releases the primary button over the middle of the node
// labeled label, the one Find returns, then settles as Settle does: the
// animations the tap starts end, but the clock does not jump to a pending
// core.StateBase.After; Advance runs that. While a dialog is open, it finds
// only what the dialog shows, so Tap("Reset") presses the dialog's Reset, not
// the page's under the scrim.
//
// A tap reaches only what a user could click. Of a node that shows in part,
// as a row half under the edge of a Scroll, it presses the middle of what
// shows, or, if something drawn above covers that, another point of it that
// nothing covers. It fails the test, saying where the node is, if there is no
// such node, if the node lies outside the window, if the clips of its
// ancestors (core.Clipper) hide it whole, or if something drawn above it,
// such as a Stack child that takes the pointer, covers all of what shows: a
// click there would reach that instead. The other actions that take a label
// (SecondaryTap, MiddleTap, DoubleTap, Hover, Scroll, Drag, Press, DropFiles
// and DropText) find their point in the same way.
func (tt *Tester) Tap(label string) {
	tt.t.Helper()
	tt.TapAt(tt.target("Tap", label))
}

// TapAt presses and releases the primary button at pos, in logical pixels from
// the top left, then settles. Two taps in a row are two single clicks, however
// near: DoubleTapAt makes a double click. It reaches whatever is at pos, or
// nothing, as a click there would, and fails the test only if pos lies outside
// the window; so do the other actions that take a position.
func (tt *Tester) TapAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("TapAt", pos)
	tt.click(pos, input.ButtonPrimary, 1)
}

// SecondaryTap clicks the secondary (right) button over the middle of the node
// labeled label, found as Tap finds it, then settles: what opens a context
// menu, such as overlay.ContextMenu or the OnContextMenu of a Table or Tree.
func (tt *Tester) SecondaryTap(label string) {
	tt.t.Helper()
	tt.SecondaryTapAt(tt.target("SecondaryTap", label))
}

// SecondaryTapAt clicks the secondary (right) button at pos, in logical pixels
// from the top left, then settles.
func (tt *Tester) SecondaryTapAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("SecondaryTapAt", pos)
	tt.click(pos, input.ButtonSecondary, 1)
}

// MiddleTap clicks the middle button, the wheel, over the middle of the node
// labeled label, found as Tap finds it, then settles: what closes a tab or
// opens a link in a new place, where an application listens for it.
func (tt *Tester) MiddleTap(label string) {
	tt.t.Helper()
	tt.MiddleTapAt(tt.target("MiddleTap", label))
}

// MiddleTapAt clicks the middle button at pos, in logical pixels from the top
// left, then settles.
func (tt *Tester) MiddleTapAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("MiddleTapAt", pos)
	tt.click(pos, input.ButtonMiddle, 1)
}

// DoubleTap double-clicks the primary button over the middle of the node
// labeled label, found as Tap finds it, then settles: what opens a row of a
// Tree or a GridView, or selects a word of a text field.
func (tt *Tester) DoubleTap(label string) {
	tt.t.Helper()
	tt.DoubleTapAt(tt.target("DoubleTap", label))
}

// DoubleTapAt double-clicks the primary button at pos, in logical pixels from
// the top left, then settles. The first press arrives as a click of its own,
// with Clicks 1, and the second with Clicks 2, as from a real mouse.
func (tt *Tester) DoubleTapAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("DoubleTapAt", pos)
	tt.click(pos, input.ButtonPrimary, 2)
}

// click presses and releases button at pos clicks times in a row, then
// settles. The pointer clock starts each click well after the last one, so
// that separate clicks never count as one double click, and the presses of
// one click follow closely.
func (tt *Tester) click(pos geom.Offset, button input.Button, clicks int) {
	tt.pointerTime += clickGap
	mods := tt.held
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Modifiers: mods, Time: tt.pointerTime})
	for range clicks {
		tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: button, Modifiers: mods, Time: tt.pointerTime})
		tt.pointerTime += pressTime
		tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: button, Modifiers: mods, Time: tt.pointerTime})
		tt.pointerTime += pressTime
	}
	tt.Pump()
	tt.Settle()
}

// Type sends s to the focused widget, as committed text from the keyboard or
// an input method, then settles.
func (tt *Tester) Type(s string) {
	tt.t.Helper()
	tt.tree.DispatchText(s)
	tt.Pump()
	tt.Settle()
}

// Key sends a key press to the focused widget, with mods and what Hold keeps
// down, then settles. It sends only the press and types nothing, as the text
// of a key comes from the keyboard or an input method on its own: Space
// presses a focused button, but puts no space in a text field. Type(" ")
// does.
func (tt *Tester) Key(k input.KeyCode, mods input.Modifiers) {
	tt.t.Helper()
	tt.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods | tt.held})
	tt.Pump()
	tt.Settle()
}

// Screenshot draws the current frame on the CPU, at Options.Scale, and returns
// it. It draws with the same shader as the GPU backends, so the picture matches
// a window to within rounding.
func (tt *Tester) Screenshot() *image.NRGBA {
	tt.t.Helper()
	img, err := tt.draw()
	if err != nil {
		tt.t.Fatalf("gowakutest: screenshot: %v", err)
	}
	return img
}

func (tt *Tester) draw() (*image.NRGBA, error) {
	if tt.device == nil {
		tt.device = soft.New()
		tt.resources = render.NewResources(text.NewAtlas(tt.shaper), tt.device)
	}
	width := int(tt.size.W*tt.scale + 0.5)
	height := int(tt.size.H*tt.scale + 0.5)
	surface, err := tt.device.CreateSurface(nil, width, height, tt.scale)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()
	tt.tree.MarkNeedsPaint()
	tt.frame()
	render.Encode(tt.canvas.DisplayList(), tt.resources, tt.scale, &tt.batch)
	if err := tt.resources.Finish(&tt.batch); err != nil {
		return nil, err
	}
	surface.SetMultisample(tt.batch.Multisampled())
	if err := tt.device.BeginFrame(surface, tt.theme.Color.Canvas); err != nil {
		return nil, err
	}
	if err := tt.device.Draw(surface, &tt.batch); err != nil {
		return nil, err
	}
	img, err := tt.device.ReadPixels(surface)
	if err != nil {
		return nil, err
	}
	return img, tt.device.EndFrame(surface)
}

// SaveScreenshot writes a screenshot to path as PNG, making its directory if
// need be.
func (tt *Tester) SaveScreenshot(path string) {
	tt.t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, tt.Screenshot()); err != nil {
		tt.t.Fatalf("gowakutest: encode %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tt.t.Fatalf("gowakutest: %v", err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		tt.t.Fatalf("gowakutest: %v", err)
	}
}

// WaitUntil waits, in real time, for work that other goroutines post through
// core.StateBase.Poster, as a search of a large file does, until done reports
// true. It pumps a frame at once and again each time work is posted, and
// checks done after each, so that the posted functions run as a window would
// run them; the fake clock does not move. It fails the test if done is still
// false after timeout, or after 10 seconds for a timeout of zero or less.
func (tt *Tester) WaitUntil(done func() bool, timeout time.Duration) {
	tt.t.Helper()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		tt.Pump()
		if done() {
			return
		}
		select {
		case <-tt.posted:
		case <-deadline.C:
			tt.Pump()
			if !done() {
				tt.t.Fatalf("gowakutest: WaitUntil gave up after %v", timeout)
			}
			return
		}
	}
}
