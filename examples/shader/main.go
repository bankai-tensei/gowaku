// Command shader shows custom shaders: an aurora that CustomPaint draws, and a
// shimmer and ripples that ShaderMask lays over text and a picture. Each is one
// HLSL function in examples/shader/shaders, compiled by cmd/gowaku-shader for
// Direct3D 11, Vulkan and Metal, with a Go fallback for the CPU renderer. See
// docs/ADR/0045-shaders-svg-animated-images.md.
//
//	shader                         animates
//	shader -png out.png -t 1.25    draws the picture at 1.25 seconds and exits
//	shader -png out.png -vulkan    the same with Vulkan on Windows
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/internal/shot"
)

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	at := flag.Float64("t", 1.25, "the time of the frame that -png writes, in seconds")
	vulkan := flag.Bool("vulkan", false, "with -png, draw with Vulkan on Windows, on the high-performance GPU")
	flag.Parse()
	picture := shaders.Picture()
	if *pngPath != "" {
		t := time.Duration(*at * float64(time.Second))
		opts := shot.Options{Width: shaders.SceneWidth, Height: shaders.SceneHeight, Theme: shot.DefaultTheme(), Vulkan: *vulkan}
		if *vulkan {
			opts.Power = gpu.HighPerformance
		}
		if err := screenshot(shaders.Scene(t, false, picture), opts, *pngPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	err := gowaku.Run(gowaku.App{
		Title: "gowaku: shader", Width: shaders.SceneWidth, Height: shaders.SceneHeight,
		Home:        shaders.Scene(0, true, picture),
		OnDrawError: func(err error) { log.Printf("draw error: %v", err) },
	})
	if err != nil {
		log.Fatal(err)
	}
}

func screenshot(root gowaku.Widget, opts shot.Options, path string) error {
	h, err := shot.Open(root, opts)
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.Settle(); err != nil {
		return err
	}
	fmt.Println(h.AdapterName)
	return h.Screenshot(path)
}
