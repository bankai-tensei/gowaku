// Command gio is the hello of Gio, for the binary size in docs/BENCH.md.
package main

import (
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

func main() {
	go func() {
		th := material.NewTheme()
		th.Shaper = text.NewShaper()
		w := new(app.Window)
		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				layout.Center.Layout(gtx, material.Body1(th, "こんにちは").Layout)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}
