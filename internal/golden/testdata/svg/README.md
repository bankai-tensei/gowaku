# SVG icons for the svg-icons golden

- `lucide/`: icons from lucide-static 1.70.0 (https://lucide.dev), unchanged. ISC License; some icons derive from Feather (MIT). See `lucide/LICENSE`.
- `material/`: icons from @material-symbols/svg-400 0.48.3 (Material Symbols by Google), unchanged, renamed to `<style>-<name>.svg`. Apache License 2.0. See `material/LICENSE`.
- `hand/`: written for gowaku to cover caps, joins, transforms, opacity, fill rules and nested viewports.
- `features/`: written for gowaku, one or more per feature of docs/ADR/0112-svg-structure-paint-clip.md and docs/ADR/0113-svg-filters-text.md. The svg-features golden draws them, and `svgfeatures_test.go` (`-tags svgsets`) compares them with Chrome.

Only these few dozen files are kept here. The test that reads the whole sets is `svgsets_test.go` (`-tags svgsets`); see docs/ADR/0045-shaders-svg-animated-images.md.
