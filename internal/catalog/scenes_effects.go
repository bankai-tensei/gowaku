package catalog

import (
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// Effects and painting.

// swatch is a colored card with a word, for the effect scenes.
func swatch(label string, c geom.Color) core.Widget {
	return basic.Container{
		Width: 100, Height: 64, Radius: 12, Color: c, Padding: geom.All(10),
		Child: basic.Text{Content: label, Style: text.Style{Size: 13, Weight: 600, Color: geom.RGBA(1, 1, 1, 1)}},
	}
}

func sceneBasicOpacity() core.Widget {
	accent := geom.Hex(0x2f5fd0)
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space12, Children: []core.Widget{
		swatch("1.0", accent),
		basic.Opacity{Opacity: 0.6, Child: swatch("0.6", accent)},
		basic.Opacity{Opacity: 0.3, Child: swatch("0.3", accent)},
	}}
}

// rings draws two overlapping discs.
func rings() core.Widget {
	return control.CustomPaint{Width: 120, Height: 80, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 10, Y: 10, W: 60, H: 60}, 30), paint.Paint{Color: geom.RGBA(0.1, 0.75, 0.95, 1)})
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 50, Y: 10, W: 60, H: 60}, 30), paint.Paint{Color: geom.RGBA(0.95, 0.3, 0.5, 1)})
	})}
}

func sceneBasicBlend() core.Widget {
	backdrop := basic.Container{Width: 280, Height: 80, Radius: 12, Gradient: &paint.Gradient{
		End: geom.Offset{X: 280}, StartColor: geom.Hex(0xfde047), EndColor: geom.Hex(0x10b981),
	}}
	return basic.Stack{Children: []core.Widget{
		backdrop,
		basic.Positioned{Left: 10, Child: rings()},
		basic.Positioned{Left: 150, Child: basic.Blend{Mode: paint.BlendMultiply, Child: rings()}},
	}}
}

// busy is a striped backdrop for the blur scenes.
func busy(w, h float32) core.Widget {
	stripes := make([]core.Widget, 0, 8)
	colors := []uint32{0xff453a, 0x0a84ff, 0x30d158, 0xffd60a}
	for i := range 8 {
		stripes = append(stripes, basic.Positioned{Left: float32(i) * w / 8, Child: basic.Container{
			Width: w / 16, Height: h, Color: geom.Hex(colors[i%len(colors)]),
		}})
	}
	return basic.SizedBox{Width: w, Height: h, Child: basic.Stack{Children: stripes}}
}

func sceneBasicBackdropFilter() core.Widget {
	return basic.Stack{Children: []core.Widget{
		busy(320, 120),
		basic.Positioned{Left: 40, Top: 20, Child: basic.BackdropFilter{Sigma: 12, Radius: 16, Child: basic.Container{
			Width: 240, Height: 80, Radius: 16, Color: geom.RGBA(1, 1, 1, 0.4), Padding: geom.All(16),
			Child: basic.Text{Content: "Frosted glass", Style: text.Style{Size: 16, Weight: 600, Color: geom.Hex(0x111111)}},
		}}},
	}}
}

func sceneBasicImageFiltered() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space24, Children: []core.Widget{
		basic.Image{Source: photo(), Width: 140},
		basic.ImageFiltered{Sigma: 3, Child: basic.Image{Source: photo(), Width: 140}},
	}}
}

func sceneBasicShaderMask() core.Widget {
	// shaders.Shimmer is a shader of examples/shader, made by cmd/gowaku-shader.
	shimmer := paint.ShaderParams{0.5, 0.2, 0.35, 0, 1, 1, 1, 0.85}
	return basic.ShaderMask{Shader: shaders.Shimmer, Params: shimmer, Time: 1250 * time.Millisecond, Animate: true,
		Child: basic.Text{Content: "gowaku", Style: text.Style{Size: 56, Weight: 700, Color: geom.Hex(0x2f5fd0)}},
	}
}

func sceneBasicTransform() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space28, Children: []core.Widget{
		basic.Rotate(0.2, swatch("Rotate", geom.Hex(0x2f5fd0))),
		basic.Skew(0.3, 0, swatch("Skew", geom.Hex(0x10a37f))),
		basic.Transform{Transform: geom.Scaling(1.2, 0.8), Origin: layout.AlignCenter, Child: swatch("Scale", geom.Hex(0xd04a2f))},
	}}
}

func sceneBasicTranslate() core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
		tile("In place", 140, 32),
		basic.Translate{X: 0.3, Child: tile("X: 0.3 of its width", 140, 32)},
	}}
}

func sceneBasicClipRRect() core.Widget {
	// A sky with a sun that reaches past its box.
	picture := basic.Stack{Children: []core.Widget{
		basic.Container{Width: 140, Height: 90, Gradient: &paint.Gradient{
			End: geom.Offset{Y: 90}, StartColor: geom.Hex(0x2f5fd0), EndColor: geom.Hex(0x9cc3f5),
		}},
		basic.Positioned{Left: 100, Top: -20, Child: basic.Container{Width: 60, Height: 60, Radius: 30, Color: geom.Hex(0xfde047)}},
	}}
	// catalog:begin
	return basic.Row{Size: layout.MainMin, Spacing: 40, Children: []core.Widget{
		picture,
		basic.ClipRect{Child: picture},
		basic.ClipRRect{Child: picture},
	}}
	// catalog:end
}

func sceneBasicScale() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space24, Children: []core.Widget{
		tile("1.0", 80, 48),
		basic.Scale{Scale: 0.8, Origin: layout.AlignCenter, Child: tile("0.8", 80, 48)},
		basic.Scale{Scale: 1.2, Origin: layout.AlignCenter, Child: tile("1.2", 80, 48)},
	}}
}

func sceneControlCustomPaint() core.Widget {
	// catalog:begin
	return control.CustomPaint{Width: 240, Height: 100, Painter: layout.PainterFunc(func(cv *paint.Canvas, sz geom.Size) {
		bars := []float32{0.4, 0.7, 0.5, 0.9, 0.6, 0.8}
		w := sz.W / float32(len(bars))
		for i, v := range bars {
			r := geom.Rect{X: float32(i)*w + 4, Y: sz.H * (1 - v), W: w - 8, H: sz.H * v}
			cv.DrawRRect(geom.RRectAll(r, 4), paint.Paint{Color: geom.Hex(0x2f5fd0).WithAlpha(0.4 + v*0.6)})
		}
		var line paint.Path
		line.MoveTo(0, sz.H*0.7)
		line.QuadTo(sz.W/2, 0, sz.W, sz.H*0.4)
		cv.DrawPath(&line, paint.Paint{Stroke: &paint.Stroke{Width: 2, Color: geom.Hex(0xd04a2f)}})
	})}
	// catalog:end
}

func sceneBasicRepaintBoundary() core.Widget {
	// A meter that changes on every frame keeps its repaints to itself.
	return basic.RepaintBoundary{Child: basic.SizedBox{Width: 200, Child: control.ProgressBar{Value: 0.45, Label: "Level"}}}
}

func sceneInteractFocusRing() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return interact.FocusRing{Visible: true, Radius: th.Radius.Control, Child: basic.Container{
			Width: 140, Height: 36, Radius: th.Radius.Control, Color: th.Color.AccentTint,
			Child: basic.Center{Child: basic.Text{Content: "Focus ring", Style: colored(th.Text.Control, th.Color.AccentInk)}},
		}}
	})
	// catalog:end
}

// Animation. The pictures show the end of each motion.

func sceneBasicAnimatedContainer() core.Widget {
	expanded := true // a field of the State; SetState changes it
	w, c := float32(120), geom.Hex(0x8a8a8e)
	if expanded {
		w, c = 220, geom.Hex(0x2f5fd0)
	}
	return basic.AnimatedContainer{Duration: 250 * time.Millisecond, Width: w, Height: 48, Radius: 24, Color: c}
}

func sceneBasicAnimatedOpacity() core.Widget {
	visible := false // a field of the State
	var o float32 = 1
	if !visible {
		o = 0.4
	}
	return basic.AnimatedOpacity{Opacity: o, Child: swatch("Fading", geom.Hex(0x2f5fd0))}
}

func sceneBasicAnimatedAlign() core.Widget {
	right := true // a field of the State
	a := layout.AlignCenterLeft
	if right {
		a = layout.AlignCenterRight
	}
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{Width: 280, Height: 56, Radius: 28, Color: th.Color.Fill, Padding: geom.All(4),
			Child: basic.AnimatedAlign{Alignment: a, Child: basic.Container{Width: 48, Height: 48, Radius: 24, Color: th.Color.Accent}}}
	})
	// catalog:end
}

func sceneBasicAnimatedPositioned() core.Widget {
	moved := true // a field of the State
	var left float32 = 8
	if moved {
		left = 160
	}
	return basic.SizedBox{Width: 260, Height: 80, Child: basic.Stack{Children: []core.Widget{
		tile("", 260, 80),
		basic.AnimatedPositioned{Left: left, Top: 16, Child: swatch("Moves", geom.Hex(0x2f5fd0))},
	}}}
}

func sceneBasicAnimatedSwitcher() core.Widget {
	page := 1 // a field of the State
	return basic.AnimatedSwitcher{
		Duration:   250 * time.Millisecond,
		Transition: basic.SlideTransition(0.04, 0),
		Child:      core.Keyed(page, tile("Page "+strconv.Itoa(page+1), 200, 64)),
	}
}

func sceneNavHero() core.Widget {
	icon := basic.Container{Width: 28, Height: 28, Radius: 7, Color: geom.Hex(0x2f5fd0)}
	big := basic.Container{Width: 56, Height: 56, Radius: 14, Color: geom.Hex(0x2f5fd0)}
	// catalog:begin
	// In the row of the list:
	row := basic.Row{Spacing: theme.Space12, Children: []core.Widget{nav.Hero{Tag: 1, Child: icon}, basic.Text{Content: "Trip to Kyoto"}}}
	// On the page the row opens, with the same Tag:
	header := basic.Row{Spacing: theme.Space12, Children: []core.Widget{nav.Hero{Tag: 1, Child: big}, basic.Text{Content: "Trip to Kyoto"}}}
	// catalog:end
	return basic.Column{Children: []core.Widget{row, header}}
}

// Input handling and accessibility: no pictures.

func sceneInteractGestureDetector() core.Widget {
	return interact.GestureDetector{
		OnTap:            func() { /* select */ },
		OnDoubleTap:      func() { /* open */ },
		OnSecondaryTapAt: func(layout.Tap) { /* a menu where the press was */ },
		OnLongPress:      func(_ geom.Offset, _ input.PointerDevice) { /* more choices */ },
		OnDragUpdate:     func(_, _ geom.Offset) { /* move by the delta */ },
		Child:            tile("Tap me", 120, 40),
	}
}

func sceneInteractMouseRegion() core.Widget {
	return interact.MouseRegion{
		Cursor:  input.CursorHand,
		OnEnter: func() { /* show the controls */ },
		OnExit:  func() { /* hide them */ },
		Child:   tile("Hover", 120, 40),
	}
}

func sceneInteractListener() core.Widget {
	return interact.Listener{
		OnPointer: func(e *core.PointerEvent) {
			switch e.Kind {
			case core.PointerDown:
				// start a stroke at e.Pos, as wide as e.Pressure for a pen
			case core.PointerMove:
				// add e.Pos to the stroke while one is going
			}
		},
		Child: basic.SizedBox{Width: 300, Height: 200},
	}
}

func sceneInteractFocus() core.Widget {
	return interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			return e.Key == input.KeySpace // handled; false passes it on
		},
		OnShowFocus: func(_ bool) { /* show a FocusRing while true */ },
		Child:       tile("Focusable", 120, 40),
	}
}

func sceneBasicIgnorePointer() core.Widget {
	return basic.Stack{Children: []core.Widget{
		control.Button{Label: "Still clickable", OnPress: func() {}},
		basic.IgnorePointer{Child: basic.Container{Width: 160, Height: 30, Color: geom.RGBA(1, 1, 1, 0.2)}},
	}}
}

func sceneInteractDraggable() core.Widget {
	return interact.Draggable{
		Data:      input.DragData{Text: "notes.txt"},
		Allowed:   input.DropCopy | input.DropMove,
		OnDragEnd: func(_ input.DropEffect) { /* remove the row on DropMove */ },
		Child:     tile("notes.txt", 120, 32),
	}
}

func sceneInteractDragTarget() core.Widget {
	return interact.DragTarget{
		Kinds:   input.DataFiles,
		OnHover: func(_ bool) { /* light up while true */ },
		OnDrop:  func(_ input.DragData, _ geom.Offset) { /* open the files */ },
		Child:   tile("Drop files here", 240, 120),
	}
}

func sceneInteractSemantics() core.Widget {
	return interact.Semantics{
		Role: semantics.RoleButton, Label: "Play", Hint: "Plays the album from the start",
		OnTap: func() { /* play */ },
		Child: basic.Icon{Path: triangleIcon()},
	}
}

func sceneInteractExcludeSemantics() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
		// The initial only decorates; the name beside it is read.
		interact.ExcludeSemantics{Child: basic.Text{Content: "S"}},
		basic.Text{Content: "Sato Hana"},
	}}
}

func sceneInteractFocusTraversalOrder() core.Widget {
	return basic.Column{Children: []core.Widget{
		field.Input{Label: "Details", OnChange: func(string) {}},
		// Tab reaches the search field first, wherever it is painted.
		interact.FocusTraversalOrder{Order: 1, Child: field.Search{OnChange: func(string) {}}},
	}}
}

// Tree services.

func sceneBasicBuilder() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		return basic.Text{Content: "Builder reads the theme above it", Style: colored(th.Text.Title, th.Color.AccentInk)}
	}}
}

func sceneBasicTheme() core.Widget {
	// catalog:begin
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		green := theme.FromAccent(geom.Hex(0x1f8a5b)).For(basic.MediaQueryOf(ctx).Brightness)
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
			control.Button{Label: "Window accent", Style: control.ButtonPrimary, OnPress: func() {}},
			basic.Theme{Data: green, Child: basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
				control.Button{Label: "Green accent", Style: control.ButtonPrimary, OnPress: func() {}},
				control.Switch{Value: true, Label: "Green", OnChange: func(bool) {}},
			}}},
		}}
	}}
	// catalog:end
}

func sceneBasicMediaQuery() core.Widget {
	// catalog:begin
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		data := basic.MediaQueryOf(ctx)
		data.Locale = language.BritishEnglish // weeks start on Monday
		return basic.MediaQuery{Data: data, Child: field.Calendar{
			Value: time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC), OnChange: func(time.Time) {},
		}}
	}}
	// catalog:end
}

func sceneBasicLocalizations() core.Widget {
	german := &basic.Strings{Language: language.German, Today: "Heute", Clear: "Löschen"}
	return basic.Localizations{Strings: []*basic.Strings{german}, Child: field.DatePicker{
		ShowToday: true, ShowClear: true, OnChange: func(time.Time) {},
	}}
}

func sceneOverlayOverlay() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		return control.Button{Label: "Show a hint", OnPress: func() {
			var entry *overlay.Entry
			entry = overlay.Of(ctx).Insert(func() core.Widget {
				return basic.Positioned{Left: 16, Top: 16, Child: control.Button{Label: "Close the hint", OnPress: func() { entry.Remove() }}}
			})
		}}
	}}
}
