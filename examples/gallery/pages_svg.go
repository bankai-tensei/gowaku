package main

import (
	"log"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of what SVG draws beyond icons: gradients, dashes and markers,
// clip paths and masks, filters and text (ADR 0112 and 0113).

var svgSamples = []struct{ label, doc string }{
	{"グラデーション", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160">
  <defs>
    <radialGradient id="ball" fx="0.35" fy="0.3"><stop offset="0" stop-color="#fff"/><stop offset="0.45" stop-color="#64d2ff"/><stop offset="1" stop-color="#0040dd"/></radialGradient>
    <linearGradient id="band" x1="0.42" x2="0.58" spreadMethod="reflect"><stop offset="0" stop-color="#ff375f"/><stop offset="1" stop-color="#ffd60a"/></linearGradient>
  </defs>
  <rect x="8" y="104" width="144" height="40" rx="10" fill="url(#band)"/>
  <circle cx="80" cy="56" r="44" fill="url(#ball)"/>
</svg>`},
	{"破線とマーカー", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160">
  <defs><marker id="a" viewBox="0 0 10 10" refX="5" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="#ff453a"/></marker>
  <marker id="d" viewBox="0 0 10 10" refX="5" refY="5" markerWidth="4" markerHeight="4"><circle cx="5" cy="5" r="5" fill="#0a84ff"/></marker></defs>
  <path d="M20 40 H140" stroke="#1d1d1f" stroke-width="3" marker-start="url(#a)" marker-end="url(#a)"/>
  <polyline points="20,120 55,70 90,120 125,70" fill="none" stroke="#636366" stroke-width="3" stroke-dasharray="8 5" marker-mid="url(#d)" marker-end="url(#a)"/>
  <circle cx="80" cy="95" r="58" fill="none" stroke="#30d158" stroke-width="4" stroke-dasharray="0 12" stroke-linecap="round"/>
</svg>`},
	{"クリップとマスク", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160">
  <defs>
    <clipPath id="c"><path d="M80 12 L98 58 L148 60 L108 90 L122 140 L80 112 L38 140 L52 90 L12 60 L62 58 Z"/></clipPath>
    <linearGradient id="f" x2="0" y2="1"><stop offset="0" stop-color="white"/><stop offset="1" stop-color="black"/></linearGradient>
    <mask id="m"><rect width="160" height="160" fill="url(#f)"/></mask>
  </defs>
  <g clip-path="url(#c)" mask="url(#m)">
    <rect width="160" height="160" fill="#bf5af2"/>
    <circle cx="80" cy="80" r="34" fill="#ffd60a"/>
  </g>
</svg>`},
	{"フィルター", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160">
  <defs>
    <filter id="s" x="-20%" y="-20%" width="150%" height="150%"><feDropShadow dx="4" dy="6" stdDeviation="5" flood-opacity="0.35"/></filter>
    <filter id="b" x="-40%" y="-40%" width="180%" height="180%"><feGaussianBlur stdDeviation="6"/></filter>
  </defs>
  <circle cx="110" cy="50" r="26" fill="#ff9f0a" filter="url(#b)"/>
  <rect x="24" y="64" width="88" height="70" rx="14" fill="#0a84ff" filter="url(#s)"/>
</svg>`},
	{"文字", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160">
  <defs><linearGradient id="t" x2="1"><stop offset="0" stop-color="#ff375f"/><stop offset="1" stop-color="#5e5ce6"/></linearGradient></defs>
  <g font-family="sans-serif" text-anchor="middle">
    <text x="80" y="60" font-size="34" font-weight="bold" fill="url(#t)">gowaku</text>
    <text x="80" y="96" font-size="15" fill="#636366">SVG の<tspan fill="#0a84ff" font-weight="bold">文字</tspan></text>
    <text x="80" y="132" font-size="18" fill="none" stroke="#30d158" stroke-width="1" letter-spacing="2">outline</text>
  </g>
</svg>`},
}

// pageSVG returns the page of SVG drawings.
func pageSVG() core.Widget { return svgPage{} }

type svgPage struct{ core.StatefulBase }

type svgPageState struct {
	core.StateBase
	drawings []*paint.SVG
}

func (svgPage) CreateState() core.State { return &svgPageState{} }

func (s *svgPageState) InitState() {
	for _, sample := range svgSamples {
		d, err := paint.ParseSVG([]byte(sample.doc))
		if err != nil {
			log.Fatal(err)
		}
		s.drawings = append(s.drawings, d)
	}
}

func (s *svgPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	caption := th.Text.Caption
	caption.Color = th.Color.Ink2
	cells := make([]core.Widget, len(s.drawings))
	for i, d := range s.drawings {
		cells[i] = basic.Column{Size: layout.MainMin, Spacing: 6, Cross: layout.CrossCenter, Children: []core.Widget{
			basic.SVG{Data: d, Size: 160, Label: svgSamples[i].label},
			basic.Text{Content: svgSamples[i].label, Style: caption},
		}}
	}
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
		heading(th, "SVG の描画", "グラデーション、破線とマーカー、クリップとマスク、フィルター、文字。"),
		collection.Group{Title: "描けるもの", Children: []core.Widget{
			basic.Pad(theme.Space16, basic.Wrap{Spacing: theme.Space16, RunSpacing: theme.Space16, Children: cells}),
		}},
	}}
}
