# はじめに

[ガイドの目次](README.md) · [English](../en/getting-started.md)

gowaku をインストールしてウィンドウを開き、数を数えるボタンを追加し、リポジトリのサンプルを動かすところまでを説明する。

## 動作環境

- **Go 1.27.3 以降**（`go.mod` の指定による）。
- **C コンパイラは不要。** gowaku は cgo を使わず、OS と GPU の API を純粋な Go から呼び出す。そのため `CGO_ENABLED=0` でビルドでき、どの OS からでも他の OS 向けにクロスコンパイルできる。
- **次のいずれかのシステム:**

| OS | GPU API | 必要なもの |
|---|---|---|
| Windows | Direct3D 11 | ハードウェアの GPU。ソフトウェアのアダプタ（WARP）は使わない。 |
| Linux | Vulkan | Vulkan のドライバ（`libvulkan.so.1` と Mesa かベンダーのドライバ）と、X11（`libX11.so.6`）か Wayland（`libwayland-client.so.0`、`libxkbcommon.so.0`）。 |
| macOS | Metal | Apple Silicon。Intel の Mac では `gowaku.Run` が `gowaku.ErrUnsupported` を返す。 |

Linux では、これらのライブラリをプログラムの起動時に読み込む。必要なのはデスクトップ環境に最初から入っているライブラリだけで、`-dev` パッケージは要らない。`WAYLAND_DISPLAY` が設定されていれば Wayland を、なければ X11 を使う。`GOWAKU_BACKEND=x11` または `GOWAKU_BACKEND=wayland` で固定することもできる。OS ごとの詳細と既知の制限は[プラットフォーム](platforms.md)を参照のこと。

## インストール

自分のモジュールで次のコマンドを実行する。

```bash
go get github.com/bankai-tensei/gowaku
```

Markdown の表示は、本体のモジュールの依存を少なく保つために別のモジュールになっている。使うときは追加する。

```bash
go get github.com/bankai-tensei/gowaku/markdown
```

リリース版ではなく手元に clone した gowaku を試すときは、`go.mod` からそれを指し、残りは Go に埋めさせる。

```bash
go mod edit -require=github.com/bankai-tensei/gowaku@v0.0.0 -replace=github.com/bankai-tensei/gowaku=../gowaku
go mod tidy
```

`go mod tidy` が gowaku の依存するモジュールとそのチェックサムを追加する。これを実行しないと、最初のビルドが「missing go.sum entry」で止まる。ただし、モジュールのどのファイルもまだ gowaku を import していないと、`go mod tidy` は gowaku の require を消してしまう。下のプログラムのように先に import を書いてから実行する。

## 最初のウィンドウ

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

type hello struct{ gowaku.StatelessBase }

func (hello) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.Text{Content: "Hello, gowaku"}}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "hello", Width: 400, Height: 300, Home: hello{}})
	if err != nil {
		log.Fatal(err)
	}
}
```

```bash
go run .
```

このコードで起きていること:

- `hello` はウィジェットであり、ただの構造体にすぎない。`gowaku.StatelessBase` を埋め込むと、すべてのウィジェットに必要な `Key` メソッドが備わる。`Build` は表示する内容を返す。
- `basic.Center` と `basic.Text` は `widgets/basic` にある標準ウィジェットで、これらも構造体リテラルで書き、ゼロ値のまま使える。
- `gowaku.Run` はウィンドウを開いて `Home` を表示し、最後のウィンドウが閉じるまで戻らない。`Run` を呼び出したゴルーチンが UI ゴルーチンになる。返される error は、使える GPU がないといった起動時の失敗だけを表す。

ウィンドウは OS のライト／ダーク設定に従う。何も変化がなければ何も動かず、操作のないウィンドウはフレームを描画しない。

## カウンター

何かを覚えておくウィジェットは stateful にする。ウィジェット自体はあくまで値であり、覚えておく内容は再ビルドをまたいで保持される `State` に置く。

```go
package main

import (
	"fmt"
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
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
	return basic.Center{Child: basic.Column{
		Size:    layout.MainMin,
		Spacing: theme.Space12,
		Children: []gowaku.Widget{
			basic.Text{Content: fmt.Sprintf("Count: %d", s.n), Style: th.Text.LargeTitle},
			control.Button{
				Label:   "Add one",
				Style:   control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.n++ }) },
			},
		},
	}}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "counter", Width: 400, Height: 300, Home: counter{}})
	if err != nil {
		log.Fatal(err)
	}
}
```

- `counter` は `gowaku.StatefulBase` を埋め込み、`CreateState` で新しい State を返す。
- `counterState` は `gowaku.StateBase` を埋め込む。これで `SetState`、`Context`、ライフサイクルのメソッドが使えるようになる。
- `SetState` は State を変更する関数を実行し、再ビルドを予約する。次のフレームではその State の `Build` だけが実行され、ツリーの他の部分には影響しない。
- `basic.ThemeOf(ctx)` はウィンドウのテーマを読み出す。`th.Text.LargeTitle` は文字の尺度（タイプスケール）にある文字スタイルの 1 つで、`theme.Space12` は余白のトークンである。
- `Size: layout.MainMin` を指定すると列の高さが子に合わせて縮み、`Center` で中央に配置できるようになる。`Column` は既定では使える高さをすべて使う。

`Button` は `widgets/control` にある。ウィジェットは層ごとにパッケージが分かれており、`basic`、`interact`、`control`、`overlay`、`field`、`nav`、`collection`、`editor` がある。全ウィジェットの一覧は、スクリーンショットとコードつきで[ウィジェットカタログ](../../catalog/ja/README.md)にまとめてある。

ガイドでは `gowaku.Widget` や `gowaku.BuildContext` と書き、カタログやウィジェットのパッケージの godoc では `core.Widget` や `core.BuildContext` と書いている。どちらも同じ型だ。`gowaku` パッケージが `core` のよく使う型に別名を付けているので、UI のファイルで `core` を import する必要はない。

## ウィンドウを開かずに見る

作った画面を見るのに、ウィンドウを開く必要はない。`gowakutest` パッケージはウィジェットをレイアウトして CPU で描画するので、普通のテストからスクリーンショットを保存できる。カウンターと同じ場所に `main_test.go` を置く。

```go
package main

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
)

func TestScreenshots(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt := gowakutest.New(t, counter{}, gowakutest.Options{Width: 400, Height: 300, Dark: dark})
		tt.Tap("Add one") // press the button by its label, as a user would
		name := "light.png"
		if dark {
			name = "dark.png"
		}
		tt.SaveScreenshot(name)
		for _, w := range tt.Warnings() {
			t.Error(w) // overflow, controls without a label, text of low contrast
		}
	}
}
```

`go test` を実行すると `light.png` と `dark.png` ができる。コーディングエージェントが自分の作った画面を確かめるにも、これがいちばん手早い。文字の入力、時間の進め方、各コントロールが報告する値、保存のたびに描き直すプレビューについては[テストとプレビュー](testing-and-preview.md)で説明している。

サンプルの `-png` オプションはリポジトリ内部の補助関数を使っているので、アプリでは代わりに `gowakutest` を使う。

## サンプルを動かす

リポジトリの [`examples/`](../../../examples/) に小さなプログラムがいくつかある。クローンしてリポジトリのルートから実行する。

```bash
git clone https://github.com/bankai-tensei/gowaku
cd gowaku
go run ./examples/hello
```

| サンプル | 内容 |
|---|---|
| `hello`, `counter` | 最小限のプログラム。`counter` は暗黙アニメーションも加えている。 |
| `todo` | テキスト入力欄とリストを備えた小さなアプリ。 |
| `gallery` | ほとんどのウィジェットを設定画面の形で並べたもの。 |
| `form` | 日本語入力に対応したテキスト入力欄、チェックボックス、ボタン。Tab キーでフォーカスが移る。 |
| `listview` | 1 万行のリスト。`-measured` を付けると、文字が折り返す行を表示する。 |
| `jptext`, `tategaki`, `scripts` | 日本語組版とルビ、縦書き、絵文字と右から左に書く文字。 |
| `navigator` | スライドして入ってくるページと Hero のアニメーション。 |
| `menus`, `tabs`, `multiwindow` | メニューバーとショートカット、別ウィンドウに切り離せるタブ、State を共有する複数のウィンドウ。 |
| `rotate`, `shader`, `glass` | アニメーションコントローラで回転させる変形、独自のシェーダー、背景のぼかし。 |
| `dragdrop`, `zoom`, `decorations` | ドラッグ＆ドロップ、トレイと通知。ピンチ操作とペン入力。ウィンドウのアイコン、サイズの制限、透過。 |
| `notes` | やや大きめのアプリ。検索、タブ、縦書き、コマンドパレットを備えたテキストのノートアプリ。 |

## 次に読むもの

- [基本の考え方](concepts.md): Widget・Element・RenderObject、State、`BuildContext`、キー、操作のないウィンドウが何も描画しない理由。
- [レイアウト](layout.md): サイズの決まり方と、レイアウト用のウィジェット。
- [テーマ](theme.md): アクセントカラー、ライトとダーク、文字スタイルとトークン。
- [モーション](motion.md): 暗黙アニメーション、コントローラ、スプリング。
- [テストとプレビュー](testing-and-preview.md): ウィンドウを開かずに `go test` の中でツリーを動かす。
- すべてのウィジェットは[ウィジェットカタログ](../../catalog/ja/README.md)にある。
