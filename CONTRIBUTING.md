# Contributing to gowaku

[日本語](CONTRIBUTING.ja.md)

Thank you for helping. Bug reports, small fixes and documentation are all welcome. For a new widget or a change to the public API, open an issue first so that the design can be agreed before the code is written.

## Before you write code

Read these two:

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): the layers and how a frame is made.
- [docs/CODING_STYLE.md](docs/CODING_STYLE.md): the rules of the code.

The rules that matter most:

- **Dependencies point down only.** `core` does not import `layout`, `gpu` or `platform`. The `depguard` linter in `.golangci.yml` enforces the layers.
- **`unsafe`, `syscall` and `purego` stay inside** `platform/win32`, `platform/x11`, `platform/wayland`, `platform/cocoa`, `gpu/d3d11`, `gpu/vulkan` and `gpu/metal`.
- **The hot paths do not allocate.** Reconcile, Layout, Paint and batching must stay at 0 allocs/op in their benchmarks.
- **Widgets are value types that work at their zero value.** No functional options, no method chains.
- **Errors and panics.** A failure to initialize returns an `error`. A broken contract while running panics with a message that starts with `gowaku: `. The library never calls `recover`.
- **No GPU needed for most tests.** `core`, `layout`, `paint`, `text`, `theme` and `motion` pass `go test ./...` without a GPU. Tests that need one carry `//go:build gpu`.

## Commands

Check and test, as CI does:

```bash
gofmt -s -l . && go vet ./... && golangci-lint run
```

```bash
GOOS=linux golangci-lint run && GOOS=darwin golangci-lint run
```

```bash
go test -race ./...
```

```bash
GOOS=linux CGO_ENABLED=0 go build ./...
```

Tests that need a GPU or a window system run on your machine only:

| Command | Where |
|---|---|
| `go test -tags gpu ./gpu/...` | Windows (D3D11 and Vulkan are compared) |
| `go test -tags gpu ./internal/golden` | Windows: D3D11 and Vulkan against the same pixel goldens |
| `go test -tags gpu ./platform/x11/` | Linux, under an X server |
| `go test -tags gpu ./platform/wayland/` | Linux, under a Wayland compositor |
| `go test -tags gpu ./gpu/metal/ ./platform/cocoa/` | macOS on Apple Silicon |

Without a Linux desktop, [scripts/wsl](scripts/wsl) runs the Linux GPU tests in WSL with a headless sway and Mesa's software Vulkan driver. Start with `getdebs.sh`, then `sway.sh`, then `gputests.sh`; each script explains itself at the top.

Generated files:

```bash
go test ./internal/golden -update        # remake the pixel goldens; look at the PNG diff before committing
go run ./internal/cmd/catalog            # remake the widget catalog (pictures and pages)
go generate ./gpu/...                    # remake the shaders
go generate ./platform/wayland/          # remake the Wayland bindings
```

`go test ./internal/catalog/` fails when the catalog is out of date. A new public widget needs an entry in `internal/catalog/registry.go` and a scene function; see [docs/catalog/README.md](docs/catalog/README.md).

## The markdown module

[markdown/](markdown) is a module of its own too. Its go.mod has `replace github.com/bankai-tensei/gowaku => ../` for work in this checkout, but a module that depends on markdown ignores that line and fetches the gowaku that markdown requires. So the require must name a version that exists and has every API markdown uses: when markdown starts to use something new from gowaku, push gowaku first, then set the require to that commit with `go get github.com/bankai-tensei/gowaku@<commit>` in `markdown/`. A release tags both, as `vX.Y.Z` and `markdown/vX.Y.Z`, with the require of markdown at `vX.Y.Z`.

## The website

[website/](website) builds the site at https://bankai-tensei.github.io/gowaku/ from the Markdown of the repository and `docs/catalog/catalog.json`. It is a module of its own, so its libraries stay out of the go.mod of gowaku. To see your changes:

```bash
cd website && go run . -serve :7901
```

The page rebuilds on every reload. A merge into main publishes it through `.github/workflows/pages.yml`. The words of the landing page are in `website/words.go`, in English and Japanese.

## Benchmarks

When you change a hot path, run the benchmarks of the package before and after, on a quiet machine, and put both numbers in the pull request:

```bash
go test -bench . -benchmem -run '^$' ./core/ ./text/ ./render/
```

## Documentation

- Doc comments are in English. Every exported identifier has one.
- User documentation is in English and Japanese: `X.md` is English and `X.ja.md` is Japanese (the guides use `docs/guide/en` and `docs/guide/ja`). Change both when you can; if you can write only one, say so in the pull request and we will translate it.
- A design decision gets an ADR in `docs/ADR/NNNN-slug.md`: background, options, decision, consequences. ADRs are written in Japanese; an English summary in the pull request is fine and we will help.

## Commits and pull requests

- Commit messages follow the Go project: `package: lowercase summary`, for example `layout: fix the baseline of a wrapped row`. Several packages: `core, layout: ...`. Everything: `all: ...`.
- One commit, one intent. Formatting-only changes go in their own commit.
- A pull request that changes what you see includes a screenshot, light and dark when it matters.
- CI must pass.

## Reporting bugs

Open an issue with the OS and its version, the GPU, the Go version, the smallest program that shows the bug, and what you expected. For text and IME bugs, add the input method and the text. For security problems, see [SECURITY.md](SECURITY.md).

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
