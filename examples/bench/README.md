# gowaku benchmarks

[日本語](README.ja.md)

The startup, idle, scroll, animation and resize scenarios are written in gowaku, Gio and Cogent Core, and measured the same way from outside. Results and how to read them: [docs/BENCH.md](../../docs/BENCH.md) ([日本語](../../docs/BENCH.ja.md)).

This directory is a separate module, so the dependencies of the compared libraries do not enter the main `go.mod`.

| Directory | Contents |
|---|---|
| `gowaku/`, `gio/`, `cogent/` | Three implementations of the same scenarios |
| `hello/` | Minimal programs for binary size. The gowaku one is `examples/hello` |
| `frames/` | Frame timestamp recording, and the contract with the harness |
| `harness/` | Launching, driving and measuring (Windows and macOS). `main_windows.go` and `main_darwin.go` hold the OS-specific parts; `result.go` and `report.go` are shared |
| `results/` | Raw measurement data |

## Running

### Windows

Only Cogent Core needs cgo. The ld of TDM-GCC 10 produces a broken PE when debug info is included, so pass `--strip-debug`. Everything is built with `-s -w` to keep the comparison fair.

```bash
mkdir bin
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gowaku.exe ./gowaku
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gio.exe ./gio
CGO_ENABLED=1 go build -ldflags="-s -w -extldflags=-Wl,--strip-debug" -o bin/cogent.exe ./cogent
CGO_ENABLED=0 go build -o bin/harness.exe ./harness
```

Pass an absolute path to `-bin`.

```bash
bin/harness.exe -bin "$PWD/bin" -out results/mine.json
bin/harness.exe -report results/mine.json
```

The full run takes about 20 minutes. During measurement the windows stay in front (hidden windows may stop drawing). They do not take focus.

### macOS

Gio and Cogent Core need cgo on macOS (Gio also fails to build `gioui.org/internal/gl` with `CGO_ENABLED=0`). Build them with Xcode's clang. gowaku stays at `CGO_ENABLED=0`.

```bash
mkdir bin
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gowaku ./gowaku
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/gio ./gio
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/cogent ./cogent
CGO_ENABLED=0 go build -o bin/harness ./harness
caffeinate -d -i bin/harness -bin "$PWD/bin" -out results/mine.json
caffeinate -d -i bin/harness -bin "$PWD/bin" -libs gowaku -scenarios dense,dense-flat -out results/mine-dense.json
bin/harness -report results/mine.json
```

Neither root nor the Accessibility permission is needed. In exchange, the harness cannot move other programs' windows or send them input.

- The default scenarios are `startup,scroll,animate,idle`. `resize` is not measured because the window size cannot be changed from outside.
- Each program opens its own window at 800×700 points (1600×1400 pixels on Retina). gowaku and Gio open at that size by default; only Cogent Core calls `SetWinSize` itself on macOS after the first frame. The position is each library's default.
- `scroll` passes `-autoscroll` to all of them. gowaku uses `ListController.JumpToIndex` every frame to place the list, pixel by pixel, at `ScrollSpeed` × elapsed time, the same as Gio. Gio and Cogent Core already advance on their own and ignore this flag.
- `caffeinate -d` keeps the display from sleeping. In the first attempt, during a period when the display was probably off, Gio and Cogent Core could not produce a first frame and gowaku drew at 112 to 120fps (that round was discarded).
- Do not leave the mouse over a window during measurement (it causes hover in `idle`).
- It takes about 20 minutes.

## Scenarios

| Name | Contents | Input |
|---|---|---|
| `startup` | From launch to the first frame. 5 times | None |
| `scroll` | A list of 10,000 rows with an icon, text and a button, for 10 seconds | 3000 logical pixels per second (equal to 30 wheel notches) |
| `animate` | 100 boxes keep changing color, size and shadow at the same time. 10 seconds | None |
| `resize` | Back and forth between 800×600 and the screen maximum. 5 seconds | The harness's `SetWindowPos` |
| `idle` | Does nothing. 60 seconds | None |
| `dense`, `dense-flat` | A 64px box moves in a window filled with static text. Only `dense` separates it with a `RepaintBoundary`. gowaku only. 10 seconds | None |

Windows are set to 1200×1050 physical pixels (800×700 at this PC's 150%). Except for `resize`, the first 2 seconds are discarded.

## What is measured

| Value | Windows | macOS |
|---|---|---|
| CPU | Difference of `GetProcessTimes`. 100% is one full core | Difference of `ri_user_time + ri_system_time` from `proc_pid_rusage` (Mach time converted with `mach_timebase_info`). 100% is one full core |
| GPU | 1-second average of `\GPU Engine(pid_N_*engtype_3D)\Utilization Percentage` | Difference of the process's `accumulatedGPUTime` (ns) from `ioreg -c AGXDeviceUserClient`, divided by wall-clock time. This is GPU time; its definition differs from Windows utilization |
| Power | `\Energy Meter(rapl_package0_pkg)\Power`. Whole-machine value, including other processes | Machine power can only be read with `powermetrics`, which needs root, so it is not measured. Instead the difference of `ri_energy_nj` (estimated energy the kernel assigns to the process's CPU time; GPU not included) goes into `ProcessWatts` |
| Memory | Peak working set from `GetProcessMemoryInfo` | `PeakMB` is `ru_maxrss` (peak resident) from `wait4` after exit. `PeakFootprintMB` is `ri_lifetime_max_phys_footprint` (maximum of the "Memory" column in Activity Monitor; includes GPU memory the process holds, such as Metal buffers) |
| Frames | Each program records a timestamp every time it draws | Same |

Frames are counted in `Paint`, so v2 gowaku does not count frames where the parent is not painted. Frames with no change are skipped, and `p99` can look like two frame periods (ADR 0019).

## What we could not align

- **Scroll input.** Only gowaku receives wheel messages. Gio reads only `WM_POINTERWHEEL`, which cannot be sent from another process. Cogent Core's wheel acts at the real mouse position. These two advance the position themselves at the same speed (3000 logical pixels per second).
- **Gio animation.** Gio has no shadow blur, so four translucent rounded rectangles are stacked instead. This may be lighter than a blur.
- **Cogent Core scrolls by row.** Its List works that way, also with the wheel. It draws slightly fewer times than the two pixel-based ones.
- **Cogent Core frame count.** Calls to `Render` are counted. Because it repaints partially, this is counted slightly differently from the "full-screen frames" of the other two.
- **GPU.** gowaku and Gio use the Iris Xe; Cogent Core uses both GPUs with its default selection. Each library was measured with its defaults.
- **4K resize.** Windows stops the window at the screen size. This PC is 1920×1200, so it cannot reach 3840×2160.
- **Not a drag.** `SetWindowPos` goes through the `WM_SIZE` path but not through the modal loop of a drag.

On macOS, these also differ:

- **Everyone scrolls on their own.** gowaku does not receive the wheel either and moves at a constant speed without inertia. What it draws differs slightly from gowaku on Windows (which glides one notch at a time with the wheel).
- **Resize is not measured.** Resizing another process's window needs the Accessibility permission.
- **GPU utilization and power are not measured.** Both need `powermetrics` (root). GPU time and the estimated process CPU power are shown instead.
- **Windows are not brought to the front.** The harness cannot change window stacking. Each program comes to the front when it starts. Do not bring other windows to the front during measurement.
