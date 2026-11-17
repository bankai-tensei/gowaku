# gowaku のベンチ

[English](README.md)

起動、アイドル、スクロール、アニメーション、リサイズのシナリオを gowaku、Gio、Cogent Core で書き、外から同じ方法で測る。結果と読み方は [docs/BENCH.ja.md](../../docs/BENCH.ja.md)（[English](../../docs/BENCH.md)）。

本体の `go.mod` に比較対象の依存が入らないよう、ここだけ別モジュールにしている。

| ディレクトリ | 内容 |
|---|---|
| `gowaku/`, `gio/`, `cogent/` | 同じシナリオの 3 実装 |
| `hello/` | バイナリサイズ用の最小プログラム。gowaku のものは `examples/hello` |
| `frames/` | フレーム時刻の記録と、ハーネスとの約束 |
| `harness/` | 起動、操作、計測（Windows と macOS）。`main_windows.go` と `main_darwin.go` が OS ごとの部分、`result.go` と `report.go` が共通 |
| `results/` | 計測の生データ |

## 実行

### Windows

Cogent Core だけ cgo が要る。TDM-GCC 10 の ld はデバッグ情報付きだと壊れた PE を作るので、`--strip-debug` を渡す。比較を揃えるため全部 `-s -w` で作る。

```bash
mkdir bin
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gowaku.exe ./gowaku
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gio.exe ./gio
CGO_ENABLED=1 go build -ldflags="-s -w -extldflags=-Wl,--strip-debug" -o bin/cogent.exe ./cogent
CGO_ENABLED=0 go build -o bin/harness.exe ./harness
```

`-bin` には絶対パスを渡す。

```bash
bin/harness.exe -bin "$PWD/bin" -out results/mine.json
bin/harness.exe -report results/mine.json
```

全部で約 20 分かかる。計測中はウィンドウが最前面に出続ける（隠れたウィンドウは描画を止めることがあるため）。フォーカスは奪わない。

### macOS

Gio と Cogent Core は macOS では cgo が要る（Gio も `CGO_ENABLED=0` では `gioui.org/internal/gl` が通らない）。Xcode の clang で作る。gowaku は `CGO_ENABLED=0` のまま。

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

root もアクセシビリティの許可も要らない。その代わり、ハーネスは他のプログラムのウィンドウを動かせず、入力も送れない。

- 既定のシナリオは `startup,scroll,animate,idle`。`resize` は外からウィンドウの大きさを変えられないので測らない。
- ウィンドウは各プログラムが自分で 800×700 ポイント（Retina で 1600×1400 ピクセル）に開く。gowaku と Gio は元からその大きさで、Cogent Core だけ macOS では最初のフレームの後に自分で `SetWinSize` する。位置は各ライブラリの既定のまま。
- `scroll` では全員に `-autoscroll` を渡す。gowaku は `ListController.JumpToIndex` で毎フレーム、Gio と同じく `ScrollSpeed` × 経過時間の位置に画素単位で置く。Gio と Cogent Core は元から自分で進めるので、このフラグを無視する。
- `caffeinate -d` で画面のスリープを止める。最初の試行では、画面が消えていたと思われる区間で Gio と Cogent Core が最初のフレームを出せず、gowaku は 112〜120fps で描いた（その回は捨てた）。
- 計測中にマウスをウィンドウの上に置かない（`idle` でホバーが起きる）。
- 約 20 分かかる。

## シナリオ

| 名前 | 内容 | 操作 |
|---|---|---|
| `startup` | 起動から最初のフレームまで。5 回 | 無し |
| `scroll` | アイコン、文字、ボタンの行が 1 万行のリストを 10 秒 | 毎秒 3000 論理ピクセル（30 ノッチのホイール相当） |
| `animate` | 100 個の箱が色、大きさ、影を同時に変え続ける。10 秒 | 無し |
| `resize` | 800×600 と画面の最大の間を往復。5 秒 | ハーネスの `SetWindowPos` |
| `idle` | 何もしない。60 秒 | 無し |
| `dense`、`dense-flat` | 動かない文字で埋まったウィンドウの中で 64px の箱が動く。`dense` だけ `RepaintBoundary` で分ける。gowaku のみ。10 秒 | 無し |

ウィンドウは物理 1200×1050 ピクセル（この PC の 150% で 800×700）に揃える。`resize` 以外は開始 2 秒を捨てる。

## 測るもの

| 値 | Windows | macOS |
|---|---|---|
| CPU | `GetProcessTimes` の差。1 コアを 100% とする | `proc_pid_rusage` の `ri_user_time + ri_system_time` の差（Mach 時間を `mach_timebase_info` で換算）。1 コアを 100% とする |
| GPU | `\GPU Engine(pid_N_*engtype_3D)\Utilization Percentage` の 1 秒ごとの平均 | `ioreg -c AGXDeviceUserClient` の、そのプロセスの `accumulatedGPUTime`（ns）の差を壁時計で割る。GPU の時間で、Windows の使用率とは定義が違う |
| 電力 | `\Energy Meter(rapl_package0_pkg)\Power`。マシン全体の値で、他のプロセスも含む | マシンの電力は root の要る `powermetrics` でしか読めないので測らない。代わりに `ri_energy_nj` の差（カーネルがそのプロセスの CPU 時間に付ける推定エネルギー。GPU は含まない）を `ProcessWatts` に入れる |
| メモリ | `GetProcessMemoryInfo` のピークワーキングセット | `PeakMB` は終了後の `wait4` の `ru_maxrss`（ピーク常駐）。`PeakFootprintMB` は `ri_lifetime_max_phys_footprint`（アクティビティモニタの「メモリ」の最大。Metal のバッファなどプロセスが持つ GPU のメモリを含む） |
| フレーム | 各プログラムが描くたびに時刻を記録する | 同じ |

`Paint` で数えるので、v2 の gowaku は親を塗らないフレームを数えない。変化の無いフレームが抜けて、`p99` が 2 フレーム分に見えることがある（ADR 0019）。

## 揃えられなかった点

- **スクロールの操作。** gowaku だけがホイールのメッセージを受ける。Gio は `WM_POINTERWHEEL` だけを読み、これは別プロセスから送れない。Cogent Core のホイールは実際のマウスの位置に効く。この 2 つは同じ速さ（毎秒 3000 論理ピクセル）で位置を自分で進める。
- **Gio のアニメーション。** Gio には影のぼかしが無いので、半透明の角丸を 4 枚重ねて代用した。ぼかしより軽い処理になる可能性がある。
- **Cogent Core のスクロールは行単位。** List がそういう作りで、ホイールでも同じ。画素単位の 2 つより描く回数が少し減る。
- **Cogent Core のフレーム数。** `Render` の呼び出しを数えている。部分再描画なので、他の 2 つの「画面全体のフレーム」とは数え方が少し違う。
- **GPU。** gowaku と Gio は Iris Xe を使い、Cogent Core は既定の選び方で両方の GPU を使う。各ライブラリの既定のまま測った。
- **4K リサイズ。** Windows はウィンドウを画面の大きさで止める。この PC は 1920×1200 なので 3840×2160 には届かない。
- **ドラッグではない。** `SetWindowPos` は `WM_SIZE` の経路を通るが、ドラッグのモーダルループは通らない。

macOS では、さらに次が違う。

- **スクロールは全員が自分で進める。** gowaku もホイールを受けず、慣性のない一定の速さで動く。Windows の gowaku（ホイールで 1 ノッチずつ滑る）とは描く中身が少し違う。
- **リサイズは測っていない。** 他のプロセスのウィンドウの大きさを変えるにはアクセシビリティの許可が要る。
- **GPU 使用率と電力は測っていない。** どちらも `powermetrics`（root）が要る。代わりに GPU 時間とプロセスの CPU の推定電力を載せる。
- **最前面にしない。** ハーネスはウィンドウの重なりを変えられない。各プログラムは起動時に前に出る。計測中に他のウィンドウを前面に出さない。
