package gowaku

import (
	"log/slog"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/semantics"
)

// maxLayoutPasses bounds how often a frame builds again after a layout.
const maxLayoutPasses = 3

// frame runs the pipeline (build, layout, paint, render) for one window. See
// docs/ARCHITECTURE.md.
func (ap *app) frame(w *window) {
	// With -tags gowakudebug, the Inspector times the stages and draws over
	// the picture; otherwise every inspecting branch is compiled out.
	var clock frameClock
	if inspecting {
		clock = ap.beginInspected(w)
	}
	w.tree.Build()
	if inspecting {
		clock.lap(stageBuild)
	}
	w.tree.Layout(geom.Tight(w.size))
	if inspecting {
		clock.lap(stageLayout)
	}
	for range maxLayoutPasses {
		if !w.tree.NeedsBuild() {
			break
		}
		w.tree.Build()
		if inspecting {
			clock.lap(stageBuild)
		}
		w.tree.Layout(geom.Tight(w.size))
		if inspecting {
			clock.lap(stageLayout)
		}
	}
	overlay := inspecting && ap.overlaid(w)
	wanted := inspecting && w.tree.NeedsRepaint()
	if overlay {
		// The overlay sits on the canvas, so the canvas is recorded afresh.
		w.tree.MarkNeedsPaint()
	}
	w.tree.Repaint(&w.canvas)
	if overlay {
		ap.drawOverlay(w)
	}
	if inspecting {
		clock.lap(stagePaint)
	}
	render.Encode(w.canvas.DisplayList(), ap.glyphs, w.scale, &w.batch)
	if err := ap.glyphs.Finish(&w.batch); err != nil {
		panic("gowaku: upload glyphs: " + err.Error())
	}
	drawErr := ap.glyphs.DrawError()
	if inspecting {
		clock.lap(stageBatch)
	}

	w.surface.SetMultisample(w.batch.Multisampled())
	if err := ap.backend.BeginFrame(w.surface, w.clearColor()); err != nil {
		panic("gowaku: begin frame: " + err.Error())
	}
	if err := ap.backend.Draw(w.surface, &w.batch); err != nil {
		panic("gowaku: draw: " + err.Error())
	}
	if err := ap.backend.EndFrame(w.surface); err != nil {
		panic("gowaku: end frame: " + err.Error())
	}
	if inspecting {
		clock.lap(stagePresent)
		ap.endInspected(w, &clock, wanted)
	}
	if drawErr != nil {
		w.drawError(drawErr)
	}
	w.speak()
}

// drawError reports err, which kept part of a frame from being drawn.
func (w *window) drawError(err error) {
	if w.onDrawError != nil {
		w.onDrawError(err)
		return
	}
	slog.Error("gowaku: part of a frame was not drawn", "window", w.title, "err", err)
}

// speak hands the accessibility tree to the screen readers, after a frame and
// only while one listens: building the tree allocates, and frames must not
// otherwise (ADR 0042).
func (w *window) speak() {
	if w.a11y == nil || !w.a11y.Listening() {
		return
	}
	root, focus := w.tree.SemanticsTree()
	if !w.tree.WindowFocused() {
		focus = 0
	}
	semantics.Compare(w.spoken, root, w.spokenFocus, focus, &w.diff)
	w.spoken, w.spokenFocus = root, focus
	w.a11y.Update(root, focus, &w.diff)
}
