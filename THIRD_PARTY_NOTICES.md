# Third-party notices

gowaku is licensed under the [MIT License](LICENSE). It includes or derives from the following material, under its own license.

## Included in the library

| What | Where | Source | License |
|---|---|---|---|
| HCT color math, ported to Go | `theme/cam16.go`, `theme/solver.go`, `theme/srgb.go`, `theme/palette.go` and nearby | [material-color-utilities](https://github.com/material-foundation/material-color-utilities), Copyright 2021 Google LLC | Apache License 2.0, [theme/LICENSE.material-color-utilities](theme/LICENSE.material-color-utilities) |
| Hyphenation patterns for American English | `text/hyphen/patterns/hyph-en-us.pat.txt`, `hyph-en-us.hyp.txt` | hyph-utf8, Copyright (C) 1990, 2004, 2005 Gerard D.C. Kuiken | Permissive, kept with the notice in [text/hyphen/patterns/hyph-en-us.license.txt](text/hyphen/patterns/hyph-en-us.license.txt) |

## Used only by tests

| What | Where | Source | License |
|---|---|---|---|
| Lucide icons | `internal/golden/testdata/svg/lucide/` | [lucide-static](https://lucide.dev) 1.70.0 | ISC, some icons MIT (Feather); [LICENSE](internal/golden/testdata/svg/lucide/LICENSE) |
| Material Symbols | `internal/golden/testdata/svg/material/` | @material-symbols/svg-400 0.48.3, Google | Apache License 2.0; [LICENSE](internal/golden/testdata/svg/material/LICENSE) |
| Test values of the HCT math | `theme/testdata/` | extracted from the tests of material-color-utilities | Apache License 2.0, as above |

## Go modules

gowaku depends on these modules, each under its own license: [purego](https://github.com/ebitengine/purego) (Apache 2.0), [go-text/typesetting](https://github.com/go-text/typesetting) (BSD-3-Clause / Unlicense), [godbus/dbus](https://github.com/godbus/dbus) (BSD-2-Clause), and `golang.org/x/image`, `golang.org/x/sys`, `golang.org/x/text` (BSD-3-Clause). The Go fonts, used by tests and the catalog pictures, come with `golang.org/x/image` under the BSD-3-Clause license.

## Examples

`examples/tategaki` sets the opening of Natsume Soseki's *Botchan* (1906), which is in the public domain, from Aozora Bunko.
