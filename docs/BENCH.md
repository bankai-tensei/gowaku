# Benchmark results: gowaku, Gio, Cogent Core

We wrote the same scenarios (startup, idle, scrolling 10,000 rows, 100 simultaneous animations, resizing) in three libraries and measured them the same way from outside the process. The code and the steps are in [examples/bench](../examples/bench/README.md).

Items where gowaku lost are shown as they are.

[日本語](BENCH.ja.md)

## Windows

Measured on 2026-11-10. Raw data: [results/windows-2026-11-10.json](../examples/bench/results/windows-2026-11-10.json).

### Environment

| Item | Value |
|---|---|
| PC | Windows 11, i7-12700H, Iris Xe + RTX 3050 Ti, 1920×1200 / 60Hz, scale 150% |
| Go | 1.27.3 |
| Compared with | Gio v0.10.3, Cogent Core v0.5.27 |
| Runs | Median of 3 runs. Startup only: median of 15 runs |

There is no three-way comparison on Linux. The Linux test environment has no real GPU, and comparing libraries on a software Vulkan driver means nothing. Reference values for gowaku alone are given below instead.

### Results

Bold marks the best value in the row. No bold when the difference is within measurement noise.

| Item | gowaku | Gio | Cogent Core |
|---|---:|---:|---:|
| **Startup** (ms) | **89** | 144 | 1217 |
| **Binary** hello (MB, `-s -w`) | 7.7 | 6.4 | 32.4 |
| cgo | Not needed | Not needed | **Needed** |
| **Idle** CPU (%) | 0.00 | 0.03 | 2.40 |
| Idle GPU (%) | 0.00 | 0.00 | 1.26 |
| Idle package power (W) | **3.2** | 3.5 | 3.9 |
| **10,000-row scroll** fps | 60.0 | 60.0 | 53.9 |
| Scroll CPU (%) | **9.1** | 12.7 | 28.7 |
| Scroll GPU (%) | 3.1 | 3.1 | 10.8 |
| Scroll frame interval p99 (ms) | 18.2 | 18.3 | 18.2 |
| **100 simultaneous animations** fps | 59.0 | 60.1 | 31.9 |
| Animation CPU (%) | **7.8** | 44.4 | 69.4 |
| Animation GPU (%) | 8.0 | 10.0 | 7.1 |
| Animation frame interval p99 (ms) | 33.3 | **18.0** | 50.6 |
| **Resize** sizes received / s | 50 | 60 | 96 |
| Resize frames drawn / s | 55 | **120** | 6 |
| Resize CPU (%) | **11.2** | 18.7 | 44.1 |
| Resize GPU (%) | 3.6 | 6.9 | **1.7** |
| **Peak memory** scroll (MB) | 240 | **159** | 434 |
| Peak memory resize (MB) | 256 | **176** | 497 |

- CPU: 100% is one full core. fps is capped by the 60Hz display, so the differences show up in CPU.
- For startup, only the first of the 15 runs is cold and slow. It does not affect the median.
- **Animation frame interval p99 is 33ms**: in 8 seconds, 2 to 6 frames take two frame periods. The cause is not known.
- **Power** is for the whole machine. Read it as a ratio between the three libraries within the same round.

### How to read it

**Where gowaku won**

- CPU for 100 simultaneous animations is about 1/6 of Gio and about 1/9 of Cogent Core. Note that Gio has no shadow blur; it uses translucent rounded rectangles instead.
- CPU for the 10,000-row scroll is about 70% of Gio and about 1/3 of Cogent Core.
- CPU for resizing is about 60% of Gio.
- The app code only writes target values. It does not compute anything per frame.

**Where gowaku lost** (background in [ADR 0015](ADR/0015-bench-results.md))

- Memory is higher than Gio in every item. Parsing the Japanese font keeps about 40MB of heap.
- Fewer sizes are received during resize than Gio (50 vs 60). Gio draws two frames per size; gowaku draws one. The cause is not known.

**No difference**

- Idle and scroll GPU are the same as Gio. Startup is faster than Gio, and the binary is 1.3MB larger. Cogent Core keeps using CPU and GPU even when idle.

## Large files

`examples/bigfile -report` opens a log and prints how long it took, on the same Windows machine (16 GB of memory). The editor reads only the lines in view, from a source that indexes the file on a goroutine of its own and keeps one entry for every 64 lines ([ADR 0157](ADR/0157-editor-line-source.md)). "The first lines" is the first frame after the window opened with the file, measured from the start of `gowaku.Run`; memory is the Go heap after a collection, including the 43 MB that the window holds without a file.

| | 1 GB, 12.6 million lines | 10 GB, 125 million lines |
|---|---:|---:|
| Start to the first lines | 76 to 82 ms | 80 to 84 ms |
| Every line indexed, in the background | 0.41 to 0.42 s | 9.1 to 9.8 s |
| Memory | 51 MB | 84 MB |

- The 1 GB file was in the cache of the OS; the 10 GB one did not fit there, so its indexing reads the disk, at about 1.1 GB/s. The first lines do not wait for it.
- The first run after writing the 10 GB file showed its first lines after 678 ms, while the system was still writing the file out.
- Scrolling and searching do not wait for the indexing either: the lines known so far scroll, and a search runs on its own goroutine and goes on into the lines as they come ([ADR 0154](ADR/0154-editor-background-find.md)).

## Linux (WSL) reference values

Measured on 2026-11-10. Only gowaku was measured.

**These are not GPU numbers.** gowaku runs on headless sway (Wayland, pixman) in WSL, with Vulkan on lavapipe, Mesa's CPU driver. Drawing happens on the CPU, so CPU usage is several times higher than on a real GPU. Only trends are meaningful: idle goes to 0, layers help, startup time, binary size.

Steps: [results/wsl-2026-11-10.md](../examples/bench/results/wsl-2026-11-10.md).

| Scenario | Result |
|---|---|
| Startup (from launch until after the first frame, median of 7 runs; the first run was 204ms) | 165ms |
| Binary (examples/hello, `-s -w`, linux/amd64) | 9.4MB |
| animate (100 boxes) | 59fps, CPU 191% |
| dense (with layers) | 59fps, CPU 102% |
| dense-flat (without layers) | 59fps, CPU 284% |
| idle | CPU 0.0%, 0 frames |

- scroll and resize were not measured. The harness that drives the wheel and the window size from outside exists only on Windows.
- Frames are capped by sway's headless output (60Hz).
- The gap between dense and dense-flat (CPU 102% vs 284%) points the same way as the layer effect seen on Windows and macOS.

## macOS

Measured on 2026-11-10. The same three programs were measured with the macOS harness (`harness/main_darwin.go`). Raw data: [results/macos-2026-11-10.json](../examples/bench/results/macos-2026-11-10.json). Steps: [examples/bench](../examples/bench/README.md#macos).

Neither root nor the Accessibility permission was used. Because of that, what can be measured and how the programs are driven differ from Windows, and the Windows table cannot be compared number by number. The differences are listed under "What we could not measure" and "Differences from Windows" below.

### Environment

| Item | Value |
|---|---|
| PC | Mac (Apple M5, 10-core GPU, 24GB memory), built-in Liquid Retina 2560×1664 / 60Hz, scale 2 |
| OS | macOS 27.0.1 |
| Go | 1.27.3 (darwin/arm64) |
| Build | gowaku with `CGO_ENABLED=0`. Gio and Cogent Core with `CGO_ENABLED=1` (Apple clang 21). All with `-s -w` |
| Compared with | Gio v0.10.3 (Metal), Cogent Core v0.5.27 (Metal through WebGPU) |
| Window | 800×700 points (1600×1400 pixels) for all three. Position is each library's default |
| Runs | Median of 3 runs. Startup only: median of 15 runs. Under `caffeinate -d -i` |

Other resident processes (`mediaanalysisd`, an antivirus extension, and so on) were running during the measurement. Load average was 2 to 3.

### Results

Bold marks the best value in the row. No bold when the difference is within measurement noise.

| Item | gowaku | Gio | Cogent Core |
|---|---:|---:|---:|
| **Startup** (ms) | 145 | 151 | 355 |
| **Binary** hello (MiB, `-s -w`) | 7.5 | **5.9** | 24.4 |
| cgo | **Not needed** | Needed | Needed |
| **Idle** CPU (%) | 0.00 | 0.00 | 2.21 |
| Idle GPU time (%) | 0.00 | 0.00 | 0.26 |
| Idle frame count (60 s) | 0 | 0 | 0 |
| **10,000-row scroll** fps | 60.0 | 60.0 | 57.6 |
| Scroll CPU (%) | **11.2** | 17.8 | 42.8 |
| Scroll GPU time (%) | 10.8 | **7.9** | 9.6 |
| Scroll frame interval p99 (ms) | 17.5 | 17.7 | 33.4 |
| **100 simultaneous animations** fps | 59.0 | 60.0 | 33.2 |
| Animation CPU (%) | **9.0** | 32.4 | 82.4 |
| Animation GPU time (%) | 19.9 | 27.7 | **4.1** |
| Animation frame interval p99 (ms) | 33.2 | **18.7** | 66.8 |
| Estimated process CPU power: scroll (mW) | **15** | 25 | 859 |
| Estimated process CPU power: animation (mW) | **13** | 88 | 5462 |
| **Peak memory** (resident) idle (MB) | 173 | **160** | 202 |
| Peak memory (resident) scroll (MB) | 175 | **169** | 272 |
| Peak footprint idle (MB) | 263 | **178** | 242 |
| Peak footprint scroll (MB) | 287 | **207** | 316 |
| Peak footprint animation (MB) | **234** | 414 | 281 |

- CPU: 100% is one full core. fps is capped by the 60Hz display.
- **GPU time** is the increase in `accumulatedGPUTime`, which the GPU driver counts per process, divided by wall-clock time. Its definition differs from the Windows "3D engine utilization". The value changes with GPU frequency, so read it as a ratio between the three libraries.
- **Estimated process CPU power** is the estimated energy the kernel assigns to CPU time (`ri_energy_nj`). It includes neither the GPU nor the rest of the machine. It varies a lot with core type (P / E) and frequency, so it does not match the CPU% ratio (Cogent Core uses 9 times the CPU and 400 times the power).
- **Peak memory (resident)** is `ru_maxrss`, the counterpart of the Windows peak working set. **Peak footprint** is the maximum of the "Memory" column in Activity Monitor. It includes GPU memory the process holds, such as Metal buffers.
- **Binary** is the size of `examples/hello`, `hello/gio` and `hello/cogent` built for darwin/arm64 (in units of 2^20 bytes).
- Startup ranges were 130 to 201ms for gowaku and 139 to 179ms for Gio. Read them as about the same.

### gowaku layers (`dense`)

| Scenario | fps | CPU (%) | GPU time (%) |
|---|---:|---:|---:|
| `dense` (with boundary) | 59.0 | **7.1** | 13.8 |
| `dense-flat` (without boundary) | 58.9 | 16.4 | 16.6 |

The effect points the same way as on WSL (284% → 102%).

### How to read it

**Where gowaku won**

- Scroll CPU is about 60% of Gio and about 1/4 of Cogent Core.
- Animation CPU is about 1/4 of Gio and about 1/9 of Cogent Core. GPU time is also lower than Gio (Gio stacks four rounded rectangles in place of a shadow blur).
- gowaku is the only one that needs no cgo. On macOS, Gio needs cgo too.
- Idle is 0, the same as Gio. Cogent Core keeps using 2% CPU and some GPU while doing nothing.

**Where gowaku lost**

- **Memory is higher than Gio.** 5 to 15MB more resident memory; 85MB more footprint at idle and 80MB more while scrolling. The idle footprint is even higher than Cogent Core (242MB). Only during animation is it lower than Gio (414MB).
- **Scroll GPU time is higher than Gio** (10.8% vs 7.9%).
- **Animation frame interval p99 is 33ms.** In 10 seconds, 0 to 4 frames take two frame periods. Same symptom as on Windows, and the same with `dense`. The cause is not known.
- **The binary is 1.6MiB larger than Gio.**

**No difference**

- Startup is the same as Gio. Cogent Core is faster than on Windows (1.2 s) but still takes more than twice as long.

### What we could not measure

| Item | Reason |
|---|---|
| Resize | Resizing another process's window needs the Accessibility permission. Having each program resize itself would be a different path from the Windows "`SetWindowPos` from outside", so we did not build it |
| GPU utilization (same definition as Windows) | `powermetrics` needs root. GPU time is shown instead |
| Whole-machine power | Also `powermetrics` (root). Estimated process CPU power is shown instead |
| 4K | The built-in display is 2560×1664. No external display |
| 120Hz | The display is 60Hz |

### Differences from Windows

- **Scroll input.** macOS does not allow sending input from outside (it needs the Accessibility permission). So gowaku, like Gio, places the list itself every frame at `ScrollSpeed` × elapsed time (`-autoscroll`, `ListController.JumpToIndex`). On Windows gowaku glides with wheel inertia, so what it draws is slightly different. Cogent Core scrolls by row in both cases.
- **The window is not brought to the front.** The harness cannot change window stacking. Each program comes to the front when it starts.
- **Each program decides its window size.** gowaku and Gio open at 800×700 points by default. Cogent Core opens at 60% or more of the screen to fit its content, so on macOS only it resizes itself to 800×700 after the first frame.
- **The mouse is not moved away from the window.** The Windows harness moves the window to the corner far from the mouse; on macOS it cannot. The mouse position is not controlled (the idle frame count is 0, so no hover happened).

### What happened during measurement

- The first attempt (3 rounds) was discarded. In its first round, during a period when the display was probably asleep, Gio and Cogent Core each failed to start 5 times (no first frame), and in the same period gowaku drew `scroll` and `animate` at 112 to 120fps. The display is 60Hz, so gowaku was drawing without display sync during that time. Metal hands out drawables without waiting for vsync when the window is not visible. We fixed it by not drawing invisible windows, the same as minimized ones (ADR 0083 §6). From the second attempt on, we measured under `caffeinate -d -i`.
- Even in the round we kept, Cogent Core's first `animate` run was 5fps at 18% CPU (it does not affect the median).

## Gap to the goals

| Goal | Result |
|---|---|
| CPU does not rise when resizing at 4K | **Not confirmed.** 3840×2160 was not measured (on this PC the window tops out at 1924×1170). At that size it uses 11.2% |
| 120fps with 10,000 rows | Capped because the display is 60Hz. `examples/listview -bench` without vsync reaches 1800fps (Iris Xe, 800×900) |
| 0% at idle | Achieved (Windows, macOS, WSL). Resize was not measured on macOS, so the 4K item has Windows values only |

## What we could not align

Scroll input, Gio's shadows, how Cogent Core frames are counted, and GPU selection differ slightly between the three. Details in [examples/bench](../examples/bench/README.md#what-we-could-not-align).

Power is for the whole machine and varied from 10 to 18W under the same conditions. It cannot be used for comparison, so it was removed from the table. It is kept in the raw data.
