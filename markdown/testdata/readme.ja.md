<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/gowaku-wordmark-dark.svg">
    <img src="docs/brand/gowaku-wordmark.svg" alt="gowaku" width="280">
  </picture>
</p>

<p align="center">
  <b>Go で書く、GPU ネイティブで cgo ゼロの、日本語に本気なデスクトップ UI ライブラリ。</b>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/bankai-tensei/gowaku"><img src="https://pkg.go.dev/badge/github.com/bankai-tensei/gowaku.svg" alt="Go Reference"></a>
  <a href="https://github.com/bankai-tensei/gowaku/actions/workflows/ci.yml"><img src="https://github.com/bankai-tensei/gowaku/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

<p align="center"><a href="README.md">English</a></p>

gowaku（Go + 枠）は、Flutter の Widget / Element / RenderObject の設計を Go に移植したものです。図形も文字も影も GPU で描き、変化がなければ何も動きません。

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/catalog/img/nav-docklayout-dark.png">
    <img src="docs/catalog/img/nav-docklayout-light.png" alt="パネルとタブのドックレイアウト" width="584">
  </picture>
</p>

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type counter struct{ gowaku.StatefulBase }

type counterState struct {
	gowaku.StateBase
	n int
}

func (counter) CreateState() gowaku.State { return &counterState{} }

func (s *counterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("count: %d", s.n), Style: th.Text.LargeTitle},
		// 暗黙アニメーション: 値が変わると勝手に補間される
		basic.AnimatedContainer{
			Duration: 300 * time.Millisecond,
			Curve:    motion.EaseOutCubic,
			Color:    th.Color.AccentTint.WithAlpha(float32(s.n%10) / 10),
			Padding:  geom.All(16),
			Radius:   th.Radius.Group,
			Child: control.Button{
				Label:   "増やす",
				Style:   control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.n++ }) },
			},
		},
	)}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "counter", Width: 400, Height: 300, Home: counter{}})
	if err != nil {
		log.Fatal(err)
	}
}
```

## インストール

```bash
go get github.com/bankai-tensei/gowaku
```

`go build` だけで動きます。C コンパイラは Windows でも Linux でも macOS でも要りません。どの OS のバイナリも他の OS からクロスコンパイルできます。実行時に各 OS で要るものは [はじめに](docs/guide/ja/getting-started.md) にあります。

| OS | GPU API | ウィンドウ |
|---|---|---|
| Windows（amd64、arm64） | Direct3D 11 | Win32 |
| Linux（amd64、arm64） | Vulkan | Wayland、X11 |
| macOS（Apple Silicon） | Metal | Cocoa |

## できること

- **純 Go からの GPU 描画。** 角丸、グラデーション、影、ぼかし、パス、クリップ、独自のシェーダー、SVG。何も変わらないアプリはフレームを描きません。
- **動き。** 暗黙アニメーション、スプリング、Hero、画面遷移が標準で入っています。
- **日本語組版。** JLReq に沿った禁則処理、約物の詰め、ぶら下げ、和欧間アキ、ルビ、縦書き。絵文字、右から左の文字、可変フォントも組めます。
- **日本語入力。** Windows、macOS、Wayland では、変換中の文字をその場に出し、注目文節を強調し、候補ウィンドウがカーソルに付いてきます。
- **アクセシビリティ。** Windows は UI Automation、Linux は AT-SPI、macOS はアクセシビリティ API に対応し、キーボードだけで操作できます。
- **独自のデザイン。** 静かなデスクトップの見た目で、ライトとダークは OS に追従します。配色は 1 つのアクセント色から作ります（[docs/DESIGN.ja.md](docs/DESIGN.ja.md)）。
- **100 を超えるウィジェット。** ボタンや入力欄から、表、ツリー、コードエディタ、Markdown の表示、ドックレイアウト、コマンドパレットまで。[ウィジェットカタログ](docs/catalog/ja/README.md) にすべて載っています。
- **デスクトップとの統合。** 複数ウィンドウ、メニュー、独自のタイトルバー、トレイアイコン、通知、クリップボード、ファイルダイアログ、ドラッグ & ドロップ。
- **型安全な API。** Widget はただの値の構造体で、ゼロ値のまま使えます。フィールドを reflect で歩かず、ビルダーも名前の文字列もありません。
- **窓なしのテスト。** `gowakutest` パッケージは、ツリーを偽の時計と CPU の描画で動かします。ラベルで部品を探し、タップや入力をし、時間を進め、スクリーンショットを撮るところまで `go test` の中で済みます。はみ出し、ラベルのない操作部品、コントラストの低い文字も警告します。
- **プレビュー。** `_test.go` に `func PreviewName() gowaku.Widget` を書いて `go run github.com/bankai-tensei/gowaku/cmd/gowaku-preview -http localhost:7878 .` を動かすと、保存するたびにライトとダークで描きます。

<table>
  <tr>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/catalog/img/collection-table-dark.png"><img src="docs/catalog/img/collection-table-light.png" alt="表"></picture></td>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/catalog/img/field-commandpalette-dark.png"><img src="docs/catalog/img/field-commandpalette-light.png" alt="コマンドパレット"></picture></td>
  </tr>
  <tr>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/catalog/img/editor-editor-dark.png"><img src="docs/catalog/img/editor-editor-light.png" alt="コードエディタ"></picture></td>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/catalog/img/field-datepicker-dark.png"><img src="docs/catalog/img/field-datepicker-light.png" alt="日付の選択"></picture></td>
  </tr>
</table>

## サンプル

```bash
go run ./examples/gallery
```

| サンプル | 内容 |
|---|---|
| `hello`, `counter` | 最小のプログラム |
| `todo`, `notes` | 小さなアプリ |
| `gallery` | 全ウィジェットをページごとに |
| `jptext`, `tategaki`, `scripts` | 日本語組版、縦書き、絵文字と右から左の文字 |
| `form`, `listview` | IME でのテキスト入力、1 万行 |
| `navigator`, `tabs` | Hero で移るページ、タブで開く文書 |
| `multiwindow`, `decorations`, `menus`, `dragdrop` | ウィンドウ、タイトルバー、メニュー、ドラッグ & ドロップ |
| `animated`, `rotate`, `zoom`, `glass`, `shader` | 動く画像、変形、ピンチでの拡大、ぼかし、独自のシェーダー |

## ベンチ

同じシナリオを 3 ライブラリで書き、Windows 11（i7-12700H、Iris Xe、60Hz）でプロセスの外から測りました。中央値で、CPU は 1 コアを 100% とします。負けた項目も載せています。方法、macOS の結果、揃えられなかった点は [docs/BENCH.ja.md](docs/BENCH.ja.md)。

| | gowaku | Gio | Cogent Core |
|---|---:|---:|---:|
| 起動から最初のフレーム（ms） | **89** | 144 | 1217 |
| バイナリ hello（MB） | 7.7 | **6.4** | 32.4 |
| cgo | 不要 | 不要（Windows） | **必要** |
| アイドル CPU（%） | 0.00 | 0.03 | 2.40 |
| 1 万行スクロール CPU（%） | **9.1** | 12.7 | 28.7 |
| 100 個同時アニメーション CPU（%） | **7.8** | 44.4 | 69.4 |
| リサイズ CPU（%） | **11.2** | 18.7 | 44.1 |
| ピークメモリ スクロール（MB） | 240 | **159** | 434 |

## 状況

まだ若いライブラリで、v1.0 までは API が変わることがあります。公開 API はルートの `gowaku` と、`core`、`layout`、`paint`、`text`、`theme`、`motion`、`semantics`、`markdown`、`geom`、`input`、`gowakutest`、`gowakupreview`、`widgets` の下のパッケージです。`gpu`、`platform`、`render` の下は、名前の上では公開でも内部扱いです（[ADR 0016](docs/ADR/0016-public-api.md)）。

分かっている制限:

- Linux は X11（XWayland、Xvfb）と Wayland（WSLg の Weston、headless の sway）で、ソフトウェアの Vulkan を使って確認しました。実 GPU のデスクトップ、GNOME と KDE では未確認です。
- X11 では、変換中の文字は入力メソッドが自分の窓に出します。Wayland のインライン表示は fcitx5 と Mozc で確認し、ibus は一部だけです。
- macOS は Apple Silicon のみです（Intel Mac は `ErrUnsupported`）。通知は .app に入れたアプリだけで使えます。

まだできていないことは [ROADMAP.ja.md](ROADMAP.ja.md) にあります。

## もっと読む

- [ガイド](docs/guide/ja/README.md): はじめに、考え方、レイアウト、テーマ、動き、文字と IME、アクセシビリティ、ウィンドウ、テスト、配布
- [ウィジェットカタログ](docs/catalog/ja/README.md): 公開ウィジェットすべての絵と例
- [API リファレンス](https://pkg.go.dev/github.com/bankai-tensei/gowaku)
- [アーキテクチャ](docs/ARCHITECTURE.ja.md)と[設計判断](docs/ADR/README.md)
- [コントリビュートの手引き](CONTRIBUTING.ja.md)

## ライセンス

[MIT](LICENSE)。同梱しているサードパーティのものは [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) にあります。
