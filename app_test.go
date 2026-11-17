package gowaku

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// fakePlatform plays back a script. Each step runs when the loop would block,
// which is the moment the application has gone idle.
type fakePlatform struct {
	lastTimeout time.Duration
	brightness  platform.Brightness
	t           *testing.T
	script      []func()
	queue       []platform.Event
	nextID      platform.WindowID
	windows     map[platform.WindowID]*fakeWindow
	modal       func()
	wakes       int
	fail        error
	// noAlpha makes the windows opaque, whatever they ask for.
	noAlpha bool

	reduceMotion bool
	locale       string
	a11y         *fakeA11y
	noTray       bool
	tray         *fakeTray
	notified     []string
}

type fakeWindow struct {
	dark   bool
	id     platform.WindowID
	closed bool
	w, h   int
	scale  float32

	imeEnabled bool
	imeRect    [4]int
	cursor     input.Cursor
	clipboard  string
	content    platform.ClipboardContent

	csd, maximized   bool
	moves, minimizes int
	menuAt           [2]int
	dialogs          []platform.FileDialogOptions
	activations      int
	hitTest          func(x, y int) platform.HitArea
	drop             func(e *platform.DropEvent) input.DropEffect
	dragged          *input.DragData
	dragDone         func(input.DropEffect)
	opts             platform.WindowOptions
	a11y             *fakeA11y
	// origin is where the client area is on the screen, known once placed.
	origin [2]int
	placed bool
	// The decorations: what SetIcon and SetSizeLimits were last given, and
	// the states.
	icon                         []*image.NRGBA
	icons                        int
	minSize, maxSize             geom.Size
	topmost, fullscreen, noAlpha bool
}

// fakeA11y records what the application hands to the screen readers.
type fakeA11y struct {
	listening bool
	updates   int
	root      *semantics.Node
	focus     semantics.ID
	diff      semantics.Diff
	// focusMoves counts the updates in which the focus moved.
	focusMoves int
}

func (a *fakeA11y) Listening() bool { return a.listening }
func (a *fakeA11y) Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff) {
	a.updates++
	a.root, a.focus = root, focus
	a.diff = semantics.Diff{Added: d.Added, Removed: d.Removed, Changed: d.Changed, FocusMoved: d.FocusMoved}
	if d.FocusMoved {
		a.focusMoves++
	}
}

func (w *fakeWindow) ID() platform.WindowID               { return w.id }
func (w *fakeWindow) PhysicalSize() (int, int)            { return w.w, w.h }
func (w *fakeWindow) Scale() float32                      { return w.scale }
func (w *fakeWindow) SetTitle(string)                     {}
func (w *fakeWindow) SetDarkTitleBar(dark bool)           { w.dark = dark }
func (w *fakeWindow) NativeHandle() platform.NativeHandle { return platform.Win32Handle{} }
func (w *fakeWindow) Close()                              { w.closed = true }
func (w *fakeWindow) SetIMEEnabled(enabled bool)          { w.imeEnabled = enabled }
func (w *fakeWindow) SetIMECursorRect(x, y, width, height int) {
	w.imeRect = [4]int{x, y, width, height}
}
func (w *fakeWindow) SetIMEVertical(bool)       {}
func (w *fakeWindow) CancelComposition()        {}
func (w *fakeWindow) SetCursor(c input.Cursor)  { w.cursor = c }
func (w *fakeWindow) ClipboardText() string     { return w.clipboard }
func (w *fakeWindow) SetClipboardText(s string) { w.clipboard = s }
func (w *fakeWindow) ClipboardKinds() platform.ClipboardKinds {
	return w.content.Kinds()
}
func (w *fakeWindow) ReadClipboard(platform.ClipboardKinds) platform.ClipboardContent {
	return w.content
}
func (w *fakeWindow) WriteClipboard(c platform.ClipboardContent) { w.content = c }
func (w *fakeWindow) SetDropHandler(fn func(e *platform.DropEvent) input.DropEffect) {
	w.drop = fn
}
func (w *fakeWindow) StartDrag(d input.DragData, _ input.DropEffect, done func(input.DropEffect)) {
	w.dragged, w.dragDone = &d, done
}
func (w *fakeWindow) ClientDecorated() bool   { return w.csd }
func (w *fakeWindow) Maximized() bool         { return w.maximized }
func (w *fakeWindow) BeginMove()              { w.moves++ }
func (w *fakeWindow) ToggleMaximize()         { w.maximized = !w.maximized }
func (w *fakeWindow) Minimize()               { w.minimizes++ }
func (w *fakeWindow) ShowWindowMenu(x, y int) { w.menuAt = [2]int{x, y} }
func (w *fakeWindow) Activate()               { w.activations++ }
func (w *fakeWindow) SetIcon(images []*image.NRGBA) {
	w.icon = images
	w.icons++
}
func (w *fakeWindow) SetSizeLimits(lo, hi geom.Size) { w.minSize, w.maxSize = lo, hi }
func (w *fakeWindow) SetAlwaysOnTop(on bool)         { w.topmost = on }
func (w *fakeWindow) SetFullscreen(on bool)          { w.fullscreen = on }
func (w *fakeWindow) Fullscreen() bool               { return w.fullscreen }
func (w *fakeWindow) Transparent() bool              { return w.opts.Transparent && !w.noAlpha }
func (w *fakeWindow) ClientOrigin() (int, int, bool) {
	return w.origin[0], w.origin[1], w.placed
}
func (w *fakeWindow) MoveClientTo(x, y int) { w.origin, w.placed = [2]int{x, y}, true }
func (w *fakeWindow) ShowsScreenPoint(x, y int) bool {
	o := w.origin
	return w.placed && x >= o[0] && y >= o[1] && x < o[0]+w.w && y < o[1]+w.h
}
func (w *fakeWindow) SetHitTest(fn func(x, y int) platform.HitArea) {
	w.hitTest = fn
}
func (w *fakeWindow) Accessibility() platform.Accessibility {
	if w.a11y == nil {
		return nil
	}
	return w.a11y
}
func (w *fakeWindow) ShowFileDialog(opts platform.FileDialogOptions, done func([]string, error)) {
	w.dialogs = append(w.dialogs, opts)
	// Before returning, as the modal dialog of Windows does. The fake platform
	// is not safe for the goroutine that the portal would use.
	done([]string{"/tmp/" + opts.Name}, nil)
}

func (p *fakePlatform) CreateWindow(opts platform.WindowOptions) (platform.Window, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	p.nextID++
	w := &fakeWindow{id: p.nextID, w: 300, h: 200, scale: 2, a11y: p.a11y, opts: opts, csd: opts.CustomFrame,
		icon: opts.Icon, minSize: opts.MinSize, maxSize: opts.MaxSize, topmost: opts.AlwaysOnTop,
		fullscreen: opts.Fullscreen, noAlpha: p.noAlpha}
	if opts.Width > 0 && opts.Height > 0 {
		w.w, w.h = int(opts.Width*2), int(opts.Height*2)
	}
	if opts.Placed {
		w.origin, w.placed = [2]int{opts.ClientX, opts.ClientY}, true
	}
	p.windows[w.id] = w
	return w, nil
}

func (p *fakePlatform) Wake() {
	p.wakes++
	p.queue = append(p.queue, platform.Event{Kind: platform.EventWake})
}

func (p *fakePlatform) SetModalHandler(fn func()) { p.modal = fn }
func (p *fakePlatform) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	if p.noTray {
		return nil, platform.ErrNoTray
	}
	p.tray = &fakeTray{opts: opts, on: on}
	return p.tray, nil
}
func (p *fakePlatform) Notify(title, body string) error {
	p.notified = append(p.notified, title+": "+body)
	return nil
}

// fakeTray records what a tray icon shows.
type fakeTray struct {
	opts   platform.TrayOptions
	on     func(platform.TrayEvent)
	closed bool
}

func (t *fakeTray) Update(opts platform.TrayOptions)    { t.opts = opts }
func (t *fakeTray) Close()                              { t.closed = true }
func (p *fakePlatform) Brightness() platform.Brightness { return p.brightness }
func (p *fakePlatform) ReduceMotion() bool              { return p.reduceMotion }
func (p *fakePlatform) Locale() string                  { return p.locale }
func (p *fakePlatform) Destroy()                        {}

func (p *fakePlatform) WaitEvent(timeout time.Duration) platform.Event {
	if timeout != 0 {
		p.lastTimeout = timeout
	}
	for {
		if len(p.queue) > 0 {
			ev := p.queue[0]
			p.queue = p.queue[1:]
			return ev
		}
		if timeout == 0 {
			return platform.Event{}
		}
		if len(p.script) == 0 {
			p.t.Fatal("the loop is blocked with nothing left in the script")
		}
		step := p.script[0]
		p.script = p.script[1:]
		step()
	}
}

func (p *fakePlatform) send(ev platform.Event) { p.queue = append(p.queue, ev) }

// fakeBackend counts frames and remembers what was drawn.
type fakeBackend struct {
	clear     geom.Color
	surfaces  int
	frames    int
	instances int
	resizes   int
	destroyed int
	// multisample is what the last frame asked for.
	multisample bool
	// refuse makes PrepareShader fail.
	refuse bool
}

type fakeSurface struct{ b *fakeBackend }

func (s *fakeSurface) Resize(int, int, float32) error { s.b.resizes++; return nil }
func (s *fakeSurface) Destroy()                       { s.b.destroyed++ }
func (s *fakeSurface) SetMultisample(on bool)         { s.b.multisample = on }

func (b *fakeBackend) CreateSurface(platform.NativeHandle, int, int, float32) (gpu.Surface, error) {
	b.surfaces++
	return &fakeSurface{b}, nil
}

func (b *fakeBackend) BeginFrame(_ gpu.Surface, clear geom.Color) error {
	b.clear = clear
	return nil
}

// Draw counts the instances that the calls draw: a call can drop its own.
func (b *fakeBackend) Draw(_ gpu.Surface, batch *gpu.Batch) error {
	b.instances = 0
	for i := range batch.Draws {
		b.instances += batch.Draws[i].Count
	}
	return nil
}

func (b *fakeBackend) EndFrame(gpu.Surface) error { b.frames++; return nil }
func (b *fakeBackend) Destroy()                   {}

func (b *fakeBackend) CreateTexture(int, int, gpu.TextureFormat) (gpu.Texture, error) {
	return fakeTexture{}, nil
}

func (b *fakeBackend) UpdateTexture(gpu.Texture, image.Rectangle, []byte, int) error { return nil }

func (b *fakeBackend) CreateTarget(int, int) (gpu.Texture, error) { return fakeTexture{}, nil }

func (b *fakeBackend) DrawToTarget(gpu.Texture, *gpu.Batch, float32) error { return nil }
func (b *fakeBackend) PrepareShader(s *gpu.Shader) error {
	if b.refuse {
		return errors.New("refused " + s.Name)
	}
	return nil
}

type fakeTexture struct{}

func (fakeTexture) Destroy() {}

func TestRun_FollowsTheOS(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var seen []theme.Brightness
	win := func() *fakeWindow { return p.windows[1] }
	home := themeProbe{seen: &seen}
	p, b = newFakes(t,
		func() {
			if win().dark {
				t.Error("a light OS must get a light title bar")
			}
			p.brightness = platform.BrightnessDark
			// Every top-level window hears of the change.
			p.send(platform.Event{Kind: platform.EventBrightness})
			p.send(platform.Event{Kind: platform.EventBrightness})
		},
		func() {
			if !win().dark {
				t.Error("a dark OS must get a dark title bar")
			}
			if got := b.clear; got != theme.Default().Dark.Color.Canvas {
				t.Errorf("the background must follow, got %+v", got)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != theme.BrightnessLight || seen[1] != theme.BrightnessDark {
		t.Errorf("got %v", seen)
	}
}

func TestRun_ThemeModeOverridesTheOS(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var seen []theme.Brightness
	p, b = newFakes(t,
		func() {
			p.brightness = platform.BrightnessDark
			p.send(platform.Event{Kind: platform.EventBrightness})
		},
		func() { p.send(platform.Event{Kind: platform.EventClose, Window: 1}) },
	)
	app := App{Home: themeProbe{seen: &seen}, ThemeMode: ThemeLight}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	for _, got := range seen {
		if got != theme.BrightnessLight {
			t.Errorf("got %v", seen)
		}
	}
}

// themeProbe records the brightness of each build.
type themeProbe struct {
	StatelessBase
	seen *[]theme.Brightness
}

func (p themeProbe) Build(ctx BuildContext) Widget {
	*p.seen = append(*p.seen, basic.ThemeOf(ctx).Brightness)
	return basic.SizedBox{}
}

func TestRun_FocusedFieldSleepsBetweenBlinks(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() {
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 1, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerDown, Window: 1, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerUp, Window: 1, X: 40, Y: 40})
		},
		func() {},
		func() {
			if p.lastTimeout <= 0 || p.lastTimeout > time.Second {
				t.Errorf("the loop must sleep until the next blink, got %v", p.lastTimeout)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: field.Input{}}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_ClickFocusesAndTypingReachesTheField(t *testing.T) {
	var got string
	var p *fakePlatform
	var b *fakeBackend
	win := func() *fakeWindow { return p.windows[1] }
	p, b = newFakes(t,
		func() {
			// The window is 150x100 logical pixels at scale 2.
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 1, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerDown, Window: 1, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerUp, Window: 1, X: 40, Y: 40})
		},
		func() {
			if !win().imeEnabled {
				t.Error("a focused text field must turn the IME on")
			}
			if win().cursor != input.CursorIBeam {
				t.Errorf("got cursor %d", win().cursor)
			}
			if win().imeRect == ([4]int{}) {
				t.Error("the IME must learn where the caret is")
			}
			p.send(platform.Event{Kind: platform.EventComposition, Window: 1, Composition: &input.Composition{Text: "か", Cursor: 3}})
			p.send(platform.Event{Kind: platform.EventComposition, Window: 1})
			p.send(platform.Event{Kind: platform.EventText, Window: 1, Text: "蚊"})
		},
		func() {
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	home := field.Input{OnChange: func(s string) { got = s }}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
	if got != "蚊" {
		t.Errorf("got %q", got)
	}
}

// A list asks for frames only while it moves.
func TestRun_TickersDriveFramesAndStop(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	idleFrames := 0
	p, b = newFakes(t,
		func() {
			idleFrames = b.frames
			p.send(platform.Event{Kind: platform.EventScroll, Window: 1, X: 20, Y: 20, ScrollY: -1})
		},
		func() {
			if b.frames <= idleFrames+1 {
				t.Errorf("the motion must take many frames, got %d", b.frames-idleFrames)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	home := collection.List{ItemCount: 100, ItemBuilder: func(int) Widget { return basic.SizedBox{} }}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
}

// counter is the shape of examples/counter, with a bar per count.
type counter struct{ StatefulBase }

type counterState struct {
	StateBase
	n int
}

var lastCounter *counterState

func (counter) CreateState() State {
	lastCounter = &counterState{}
	return lastCounter
}

func (s *counterState) Build(BuildContext) Widget {
	children := make([]Widget, s.n+1)
	for i := range children {
		children[i] = basic.Container{Width: 10, Height: 10, Color: geom.Hex(0x6750a4)}
	}
	return basic.VStack(children...)
}

func newFakes(t *testing.T, script ...func()) (*fakePlatform, *fakeBackend) {
	t.Helper()
	return &fakePlatform{t: t, script: script, windows: make(map[platform.WindowID]*fakeWindow)}, &fakeBackend{}
}

func closeWindow(p *fakePlatform, id platform.WindowID) func() {
	return func() { p.send(platform.Event{Kind: platform.EventClose, Window: id}) }
}

func TestRun_FirstFrameAndExit(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){closeWindow(p, 1)}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.frames != 1 || b.instances != 1 {
		t.Errorf("got %d frames of %d instances, want 1 of 1", b.frames, b.instances)
	}
	if !p.windows[1].closed || b.destroyed != 1 {
		t.Error("the window and its surface must be closed")
	}
}

func TestRun_IdleDrawsNothing(t *testing.T) {
	p, b := newFakes(t)
	var settled int
	p.script = []func(){
		func() { p.send(platform.Event{Kind: platform.EventRedraw, Window: 1}) },
		func() {
			settled = b.frames
			// Events that change nothing must not cost a frame.
			p.send(platform.Event{Kind: platform.EventWake})
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			if b.frames != settled {
				t.Errorf("idle loop drew %d frames", b.frames-settled)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PostAndSetStateDrawOneFrame(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			Post(func() { lastCounter.SetState(func() { lastCounter.n = 4 }) })
			Post(func() { lastCounter.SetState(func() { lastCounter.n = 5 }) })
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.frames != 2 {
		t.Errorf("got %d frames, want 2: two SetState calls share a frame", b.frames)
	}
	if b.instances != 6 {
		t.Errorf("got %d instances, want 6", b.instances)
	}
}

func TestRun_ResizeRelaysOut(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 640, PhysicalHeight: 480, Scale: 2})
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.resizes != 1 || b.frames != 2 {
		t.Errorf("got %d resizes and %d frames, want 1 and 2", b.resizes, b.frames)
	}
}

func TestRun_MinimizedWindowDrawsNothing(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() { p.send(platform.Event{Kind: platform.EventResize, Window: 1, Scale: 2}) },
		func() { Post(func() { lastCounter.SetState(func() { lastCounter.n++ }) }) },
		func() {
			if b.frames != 1 {
				t.Errorf("got %d frames, want only the first one before minimizing", b.frames)
			}
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 300, PhysicalHeight: 200, Scale: 2})
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.frames != 2 {
		t.Errorf("restoring must draw the pending change once, got %d frames in all", b.frames)
	}
}

func TestRun_ModalHandlerDrawsWhileLoopIsBlocked(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			p.send(platform.Event{Kind: platform.EventRedraw, Window: 1})
			p.modal() // the OS calling from inside its own loop
			if b.frames != 2 {
				t.Errorf("got %d frames, want the first one and one from the modal handler", b.frames)
			}
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if p.modal != nil {
		t.Error("the handler must be cleared when Run ends")
	}
}

func TestRun_SecondWindowKeepsRunning(t *testing.T) {
	p, b := newFakes(t)
	var second *WindowHandle
	p.script = []func(){
		func() {
			var err error
			second, err = OpenWindow(WindowOptions{}, counter{})
			if err != nil {
				t.Error(err)
			}
		},
		closeWindow(p, 1),
		func() {
			if b.destroyed != 1 {
				t.Errorf("got %d surfaces destroyed, want 1", b.destroyed)
			}
			Post(func() {
				second.Close()
				second.Close() // a second close is harmless
			})
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.destroyed != 2 {
		t.Errorf("got %d surfaces destroyed, want 2", b.destroyed)
	}
}

// closeOnTap closes its window from a callback of the tree that the window owns,
// and touches the tree again afterward, as the rest of a dispatch would.
type closeOnTap struct {
	StatelessBase
	handle **WindowHandle
	after  func()
}

// Build implements core.StatelessWidget.
func (c closeOnTap) Build(BuildContext) Widget {
	return interact.GestureDetector{
		OnTap: func() {
			(*c.handle).Close()
			c.after()
		},
		Child: basic.Container{Width: 150, Height: 100, Color: geom.Hex(0x808080)},
	}
}

func TestWindowHandle_CloseFromItsOwnCallback(t *testing.T) {
	p, b := newFakes(t)
	var second *WindowHandle
	destroyedInCallback := -1
	root := closeOnTap{handle: &second, after: func() { destroyedInCallback = b.destroyed }}
	p.script = []func(){
		func() {
			var err error
			if second, err = OpenWindow(WindowOptions{}, root); err != nil {
				t.Error(err)
			}
			// Returning from the wait lets the new window draw its first frame.
			p.send(platform.Event{Kind: platform.EventRedraw, Window: 2})
		},
		func() {
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 2, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerDown, Window: 2, X: 40, Y: 40})
			p.send(platform.Event{Kind: platform.EventPointerUp, Window: 2, X: 40, Y: 40})
			// Events that were already queued must not reach the closing window.
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 2, X: 40, Y: 40})
		},
		func() {
			if destroyedInCallback != 0 {
				t.Errorf("the window was gone during its own callback: %d surfaces destroyed", destroyedInCallback)
			}
			if b.destroyed != 1 {
				t.Errorf("got %d surfaces destroyed after the callback, want 1", b.destroyed)
			}
			if !p.windows[2].closed {
				t.Error("the OS window must be closed")
			}
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

// postOnInit calls Post from InitState, during the mount of the first window.
type postOnInit struct {
	StatefulBase
	fn func()
}

type postOnInitState struct{ StateBase }

// CreateState implements core.StatefulWidget.
func (postOnInit) CreateState() State { return &postOnInitState{} }

// InitState implements core.State.
func (s *postOnInitState) InitState() { Post(s.Widget().(postOnInit).fn) }

// Build implements core.State.
func (s *postOnInitState) Build(BuildContext) Widget { return basic.SizedBox{} }

func TestPost_FromInitStateOfHomeRuns(t *testing.T) {
	p, b := newFakes(t)
	ran := false
	p.script = []func(){closeWindow(p, 1)}
	if err := run(p, b, testutil.NewShaper(t), App{Home: postOnInit{fn: func() { ran = true }}}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("a Post from InitState of Home was dropped")
	}
}

func TestRun_PlainFramesDoNotMultisample(t *testing.T) {
	p, b := newFakes(t)
	b.multisample = true
	p.script = []func(){closeWindow(p, 1)}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.multisample {
		t.Error("a frame of text and quads asked for multisampling")
	}
}

func TestRun_QuitClosesEverything(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			if _, err := OpenWindow(WindowOptions{}, counter{}); err != nil {
				t.Error(err)
			}
			Quit()
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if b.destroyed != 2 {
		t.Errorf("got %d surfaces destroyed, want 2", b.destroyed)
	}
}

func TestRun_InitErrorIsReturned(t *testing.T) {
	p, b := newFakes(t)
	want := errors.New("no window")
	p.fail = want
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); !errors.Is(err, want) {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestPost_OutsideRunIsDropped(t *testing.T) {
	Post(func() { t.Error("must not run") })
	Quit()
}

func TestOpenWindow_OutsideRunPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got no panic")
		}
	}()
	_, _ = OpenWindow(WindowOptions{}, counter{}) //nolint:errcheck // the call panics
}

func TestRun_OnCloseRunsBeforeTheTreeGoes(t *testing.T) {
	p, b := newFakes(t)
	var log []string
	p.script = []func(){
		func() {
			opts := WindowOptions{OnClose: func() { log = append(log, "second") }}
			if _, err := OpenWindow(opts, counter{}); err != nil {
				t.Error(err)
			}
		},
		closeWindow(p, 1),
		func() { Quit() },
	}
	app := App{Home: counter{}, OnClose: func() { log = append(log, "first") }}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if strings.Join(log, " ") != "first second" {
		t.Errorf("got %v", log)
	}
}

func TestRun_OnDrawErrorReportsARefusedShader(t *testing.T) {
	p, b := newFakes(t)
	b.refuse = true
	p.script = []func(){closeWindow(p, 1)}
	var errs []error
	shader := paint.NewShader(paint.Shader{Name: "broken"})
	app := App{
		Home:        control.CustomPaint{Shader: shader, Width: 20, Height: 20},
		OnDrawError: func(err error) { errs = append(errs, err) },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "refused broken") {
		t.Errorf("OnDrawError got %v, want the refusal once", errs)
	}
	if b.frames != 1 || b.instances != 0 {
		t.Errorf("%d frames of %d instances, want one without the shader", b.frames, b.instances)
	}
}

func TestShowModal_BlocksTheOwnerAndClosesWithIt(t *testing.T) {
	p, b := newFakes(t)
	var ctx BuildContext
	var log []string
	var modal *WindowHandle
	owner := func() *window { return currentApp.Load().windows[1] }
	p.script = []func(){
		func() {
			var err error
			opts := WindowOptions{Title: "modal", OnClose: func() { log = append(log, "modal") }}
			if modal, err = ShowModal(ctx, opts, counter{}); err != nil {
				t.Fatal(err)
			}
			if WindowOf(ctx).window != owner() {
				t.Error("WindowOf must find the window of the context")
			}
		},
		func() {
			o := p.windows[2].opts
			if o.Owner != p.windows[1] || !o.Modal {
				t.Errorf("the platform must get the owner and Modal, got %+v", o)
			}
			p.send(platform.Event{Kind: platform.EventKeyDown, Window: 1, Key: input.KeyTab})
			p.send(platform.Event{Kind: platform.EventPointerDown, Window: 1, X: 10, Y: 10})
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
		func() {
			if owner() == nil {
				t.Fatal("a close request to the owner must wait for the modal window")
			}
			if owner().tree.Focused() != nil {
				t.Error("the owner must not take keys while the modal window is open")
			}
			if n := p.windows[2].activations; n != 2 {
				t.Errorf("a press and a close request on the owner must activate the modal window, got %d", n)
			}
			modal.Close()
			p.Wake()
		},
		func() {
			if p.windows[1].activations != 1 {
				t.Error("the owner must get the keyboard back")
			}
			p.send(platform.Event{Kind: platform.EventKeyDown, Window: 1, Key: input.KeyTab})
		},
		func() {
			if owner().tree.Focused() == nil {
				t.Error("the owner must take keys again")
			}
			if _, err := ShowModal(ctx, WindowOptions{OnClose: func() { log = append(log, "second") }}, counter{}); err != nil {
				t.Fatal(err)
			}
		},
		closeWindowNow(p, 1),
	}
	home := basic.Builder{Content: func(c BuildContext) Widget {
		ctx = c
		return control.Button{Label: "b", OnPress: func() {}}
	}}
	app := App{Home: home, OnClose: func() { log = append(log, "owner") }}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if strings.Join(log, " ") != "modal second owner" {
		t.Errorf("closing order %v, want the modal window before its owner", log)
	}
}

// closeWindowNow closes window id from the program, which a modal window does
// not hold back.
func closeWindowNow(p *fakePlatform, id platform.WindowID) func() {
	return func() {
		ap := currentApp.Load()
		(&WindowHandle{ap, ap.windows[id]}).Close()
		p.Wake()
	}
}

func TestTitleBar_HitAreasAndMenus(t *testing.T) {
	p, b := newFakes(t)
	var ranAbout bool
	bar := overlay.MenuBar{Items: []overlay.MenuItem{{
		Action: interact.Action{Label: "&Help"},
		Items:  []overlay.MenuItem{{Action: interact.Action{Label: "&About", OnInvoke: func() { ranAbout = true }}}},
	}}}
	p.script = []func(){
		func() {
			w := p.windows[1]
			if !w.opts.CustomFrame {
				t.Fatal("CustomFrame must reach the platform")
			}
			// The window is 400 by 100 logical at a scale of 2.
			area := func(x, y float32) platform.HitArea { return w.hitTest(int(x*2), int(y*2)) }
			if got := area(20, 18); got != platform.HitClient {
				t.Errorf("over the menu bar: %v, want the client", got)
			}
			if got := area(200, 18); got != platform.HitCaption {
				t.Errorf("over the empty bar: %v, want the caption", got)
			}
			if got := area(400-titleBarInset-titleButtonW*1.5-titleButtonGap, 18); got != platform.HitMaximize {
				t.Errorf("over the maximize button: %v, want it", got)
			}
			if got := area(70, 60); got != platform.HitClient {
				t.Errorf("below the bar: %v, want the client", got)
			}
			p.send(platform.Event{Kind: platform.EventKeyDown, Window: 1, Key: input.KeyH, Modifiers: input.ModAlt})
		},
		func() { p.send(platform.Event{Kind: platform.EventKeyDown, Window: 1, Key: input.KeyEnter}) },
		closeWindow(p, 1),
	}
	home := basic.Column{Children: []Widget{TitleBar{Child: bar}, basic.Text{Content: "home"}}}
	app := App{CustomFrame: true, Width: 400, Height: 100, Home: home}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if !ranAbout {
		t.Error("the menu bar in the title bar must open its menu from the keyboard")
	}
}

func TestDrag_ThroughThePlatform(t *testing.T) {
	p, b := newFakes(t)
	var dropped input.DragData
	var ended input.DropEffect
	p.script = []func(){
		func() {
			w := p.windows[1]
			// The target fills the bottom half of a 150×100 window at a scale of 2.
			over := platform.DropEvent{Phase: platform.DropOver, X: 20, Y: 160, Kinds: input.DataFiles, Allowed: input.DropCopy}
			if e := w.drop(&over); e != input.DropCopy {
				t.Errorf("over the target: %v", e)
			}
			drop := over
			drop.Phase, drop.Data = platform.DropDrop, input.DragData{Files: []string{"/x"}}
			if e := w.drop(&drop); e != input.DropCopy {
				t.Errorf("drop: %v", e)
			}
			p.send(platform.Event{Kind: platform.EventPointerDown, Window: 1, X: 20, Y: 20})
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 1, X: 60, Y: 20})
		},
		func() {
			w := p.windows[1]
			if w.dragged == nil || w.dragged.Text != "drag me" {
				t.Fatalf("the press and move must start a drag, got %v", w.dragged)
			}
			w.dragDone(input.DropCopy)
			p.Wake()
		},
		closeWindow(p, 1),
	}
	home := basic.Column{Cross: layout.CrossStretch, Children: []Widget{
		basic.Expanded{Child: interact.Draggable{
			Data: input.DragData{Text: "drag me"}, OnDragEnd: func(e input.DropEffect) { ended = e },
			Child: basic.Container{},
		}},
		basic.Expanded{Child: interact.DragTarget{
			OnDrop: func(d input.DragData, _ geom.Offset) { dropped = d },
			Child:  basic.Container{},
		}},
	}}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
	if len(dropped.Files) != 1 || dropped.Files[0] != "/x" {
		t.Errorf("dropped %v", dropped)
	}
	if ended != input.DropCopy {
		t.Errorf("the end of the drag must reach the widget, got %v", ended)
	}
}

func TestTray_MenuActivateAndNotify(t *testing.T) {
	p, b := newFakes(t)
	var log []string
	p.script = []func(){
		func() {
			ctx := lastCounter.Context()
			tr, err := ShowTray(ctx, TrayOptions{
				Tooltip:    "tip",
				OnActivate: func() { log = append(log, "activate") },
				Menu: []overlay.MenuItem{
					{Action: interact.Action{Label: "開く(&O)", OnInvoke: func() { log = append(log, "open") }}},
					{Separator: true},
					{Action: interact.Action{Label: "無効"}},
					{Action: interact.Action{Label: "表示"}, Items: []overlay.MenuItem{
						{Action: interact.Action{Label: "常に手前", Checkable: true, Checked: true, OnInvoke: func() { log = append(log, "top") }}},
					}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			items := p.tray.opts.Items
			if len(items) != 4 || !items[1].Separator || items[2].Enabled || !items[3].Enabled || !items[3].Items[0].Checked {
				t.Errorf("items %+v", items)
			}
			p.tray.on(platform.TrayEvent{Kind: platform.TrayActivate})
			p.tray.on(platform.TrayEvent{Kind: platform.TrayChoose, Item: items[0].ID})
			p.tray.on(platform.TrayEvent{Kind: platform.TrayChoose, Item: items[3].Items[0].ID})
			if err := Notify(ctx, "題", "本文"); err != nil {
				t.Error(err)
			}
			_ = tr
			p.Wake()
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(log, " ") != "activate open top" {
		t.Errorf("log %v", log)
	}
	if len(p.notified) != 1 || p.notified[0] != "題: 本文" {
		t.Errorf("notified %v", p.notified)
	}
	if !p.tray.closed {
		t.Error("the tray must go when Run ends")
	}
}
