# アクセシビリティ

[English](../en/accessibility.md)

gowaku のウィンドウは、ウィジェットのツリーと並べてアクセシビリティのツリーを管理している。各操作部品が何であり、どんな名前で、値や状態はどうなっているか、スクリーンリーダーから何ができるかを表すツリーである。gowaku はこのツリーを、Windows では UI Automation、Linux では AT-SPI2、macOS では NSAccessibility を通じて OS のスクリーンリーダーに渡す。標準のウィジェットは自身の情報を自分で提供するので、アプリの作者が受け持つのは、操作部品にラベルを付けること、小さな部品を組み合わせて作ったものに説明を与えること、キーボードで操作できる状態を保つことである。

テストで `gowakutest` の `Find` や `Tap` が探すのも、このツリーである。ラベルをきちんと付けたアプリは、テストも書きやすい。

## ラベル

名前の無い操作部品は「ボタン」としか読み上げられない。操作部品には必ずラベルを付ける。

- `control.Button` は `Label` を読み上げる。一覧の各行に同じ「削除」ボタンが並ぶ場合のように、ラベルだけでは区別できないボタンには `Hint` を加える。スクリーンリーダーはラベルの後にヒントを読み上げる。
- `field.Input` は `Label` を読み上げ、ラベルが無ければ `Placeholder` を名前にする。グループの行に名前が付いている入力欄では、どちらも不要である。
- `basic.Icon` と `basic.Image` は `Label` を読み上げる。装飾のための画像では空のままにしておく。
- `interact.Pressable` は `Label`、`Role`、`Value`、`State` を読み上げる。ラベルの無い Pressable では、中に含まれる文字列をすべて空白でつないだものが名前になる。

操作部品の中にあって、その部品のラベルと同じ文字列を表示する Text はツリーに入れない。同じボタンが二度読み上げられるのを防ぐためである。

`basic.Text` は常に読み上げられる。名前の横に置くアバターの頭文字のように、飾りにすぎない文字は `interact.ExcludeSemantics` で囲む。これは Web の `aria-hidden` と同じく配下をツリーから外し、描画とポインタの扱いはそのまま残す。中に操作部品を置いてはいけない。Tab キーでは届くが、スクリーンリーダーからは見えなくなる。

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// person は頭文字のアバターを表示する。読み上げられるのは名前だけである。
func person(ctx gowaku.BuildContext, initial, name string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		interact.ExcludeSemantics{Child: basic.Container{
			Width: 32, Height: 32, Radius: 16, Color: th.Color.AccentTint,
			Child: basic.Center{Child: basic.Text{Content: initial, Style: th.Text.Headline}},
		}},
		basic.Text{Content: name},
	}}
}
```

```go
control.Button{Label: "削除", Hint: "請求書 2026-10", OnPress: remove}
```

## 自作のウィジェットに説明を付ける

`interact.Semantics` は子ウィジェットに説明を付ける。指定できるのは、ロール、ラベル、値、ヒント、状態、ライブリージョンと、スクリーンリーダーからの要求を実行するコールバックである。

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// heading marks a title as a heading of level 1.
func heading(title string) gowaku.Widget {
	return interact.Semantics{
		Role:  semantics.RoleHeading,
		Level: 1,
		Label: title,
		Child: basic.Text{Content: title},
	}
}

// status announces its text whenever it changes, once the reader is idle.
func status(msg string) gowaku.Widget {
	return interact.Semantics{
		Live:  semantics.LivePolite,
		Label: msg,
		Child: basic.Text{Content: msg},
	}
}

// volume is a range that the screen reader can step up and down.
func volume(v float64, up, down func()) gowaku.Widget {
	return interact.Semantics{
		Role:       semantics.RoleSlider,
		Label:      "音量",
		Range:      semantics.Range{Min: 0, Max: 100, Value: v},
		OnIncrease: up,
		OnDecrease: down,
		Child:      basic.SizedBox{Width: 200, Height: 24},
	}
}
```

ロールは `semantics` パッケージで定義されており、ボタン、チェックボックス、スイッチ、ラジオボタンとそのグループ、タブ、リスト、ツリー、グリッド、メニュー、ダイアログ、見出し、リンク、スライダー、ステッパー、プログレス、コンボボックス、ツールチップなどがある（`semantics.Role`）。状態は `semantics.State` のビットで表し、`Checked`、`Mixed`、`Selected`、`Expanded`、`Collapsed`、`Disabled`、`ReadOnly`、`Multiline`、`Multiselectable`、`Invalid`、`Required`、`Obscured` がある。`Focusable` と `Focused` は gowaku が設定するので、アプリからは触らない。

コールバックは `OnTap`、`OnIncrease`、`OnDecrease`、`OnSetValue`、`OnExpand`、`OnCollapse` の 6 つで、nil のものに対応するアクションはスクリーンリーダーに公開されない。`State` に `Disabled` が含まれていれば、どのコールバックも呼ばれない。呼び出しはクリックと同じく UI ゴルーチンで行われる。

ライブリージョン（`LivePolite`、`LiveAssertive`）は、ラベルか値が変わったときに読み上げられる。表示されただけでは読み上げられない。

`semantics` パッケージには、ツリーそのもの（`semantics.Node`）と、2 つのツリーの差分を取る機能（`semantics.Compare`、`semantics.Diff`）もある。アプリから直接使うことはまれで、テストでは `gowakutest` を通じてツリーを読む。

## フォーカスとキーボード

Tab と Shift+Tab で、ウィンドウ内の操作部品の間を描画順にフォーカスが移動する。Space と Enter はフォーカスのあるボタンを押し、矢印キーはグループ、リスト、メニュー、グリッドの中を移動する。

- 操作部品をクリックすると、Windows のネイティブコントロールと同じく、その部品にフォーカスが移る。フォーカスを受け取る部品の無い場所をクリックするとフォーカスは外れる。スクロールバーをドラッグしてもフォーカスは動かない。
- `interact.FocusTraversalOrder{Order: n}` は Tab の順序を変える。HTML の `tabindex` と同じく、0 より大きい値を持つものが値の小さい順に先に来て、残りは描画順で続く。
- `interact.Focus` を使うと、どのウィジェットでもキーボード入力を受け取れるようになる。`OnKey` でキー入力を受け取り、`Autofocus` を指定すると最初に表示されたときにフォーカスを得る。
- `Pressable` の `SkipTab` を指定すると、ポインタでしか操作しない領域になる。
- モーダルダイアログは、開いている間フォーカスを自分の内側に閉じ込める。

```go
interact.FocusTraversalOrder{Order: 1, Child: field.Search{OnChange: s.search}}
```

### フォーカスリング

標準の操作部品は、フォーカスを得ると淡いリングを表示する。ただし表示するのはキーボード操作でフォーカスを得たときだけで（Web の `:focus-visible` と同じ）、マウスでクリックしたときには表示しない。自作の操作部品では、`interact.FocusRing` と `Pressable` の `FocusVisible` を使って同じリングを描ける。

```go
interact.Pressable{Label: "Album", OnPress: open, Build: func(s interact.PressState) gowaku.Widget {
	return interact.FocusRing{Visible: s.FocusVisible, Radius: 8, Child: cover}
}}
```

`interact.Focus` は同じフラグを `OnShowFocus` で通知する。リングは子の外側に描かれ、レイアウト上の場所を取らないので、周囲に数ピクセルの余白を空けておく。

## 動きを減らす

利用者が OS で動きを減らす設定を有効にすると、`basic.ReduceMotionOf(ctx)` が真になる。標準のウィジェットは移動や拡大縮小のアニメーションをやめて一度に切り替えるが、短いフェードは残す。プログレスの回転のように明示的に使っている Ticker は止めない。gowaku は各 OS の設定を読み取り、設定の変更にも追従する。

- Windows: 設定 › アクセシビリティ › 視覚効果 › アニメーション効果。
- macOS: アクセシビリティ設定の「視差効果を減らす」（Reduce motion）。
- Linux: xdg-desktop-portal 経由で読む `org.gnome.desktop.interface` の `enable-animations`。ポータルが無い場合と KDE では減らさない。

## テストで確かめる

`gowakutest.Tester.Warnings` は現在のフレームを検査し、次のような問題を報告する。

- `WarnUnlabeled`: 支援技術に何も伝えないボタン、チェックボックス、スライダー、入力欄。
- `WarnContrast`: 背景とのコントラストが低すぎる文字。WCAG 2 と同じく、4.5:1 未満（大きな文字では 3:1 未満）を対象とする。

```go
package app

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

func TestLabels(t *testing.T) {
	tt := gowakutest.New(t, control.Button{Label: "保存", OnPress: func() {}}, gowakutest.Options{})
	for _, w := range tt.Warnings() {
		t.Error(w)
	}
	if _, ok := tt.Find("保存"); !ok {
		t.Errorf("no button named 保存:\n%s", tt.Dump())
	}
}
```

`Dump` は、スクリーンリーダーから見えるとおりの形でアクセシビリティのツリーを出力する。`Options.ReduceMotion` は OS の設定の代わりに使える。`gowaku-preview` もプレビューごとに同じ検査を行う。[テストとプレビュー](testing-and-preview.md)を参照。

## OS ごとの対応

| | Windows | Linux | macOS |
|---|---|---|---|
| ブリッジ | UI Automation のプロバイダ | D-Bus 上の AT-SPI2（X11 と Wayland） | NSAccessibility |
| 確認したスクリーンリーダー | ナレーターは手動で、NVDA は読み上げログで | Orca はデバッグログで | VoiceOver はコードのテストまで |
| 入力欄 | Text パターン: 文書、選択範囲、キャレット。文字・単語・行単位の移動 | Text と EditableText: 同じ範囲 | 文字列、選択範囲、行、範囲 |

ブリッジはスクリーンリーダーから問い合わせがあるまで何もしない。スクリーンリーダーを使わない環境では、ツリーのための負荷はかからない。

**Windows。** プロバイダは Invoke、Toggle、Value、RangeValue、SelectionItem、ExpandCollapse、ScrollItem、Text の各パターンに応答し、フォーカス、プロパティ、構造、ライブリージョンのイベントを発行する。自動テストでは、Go で書いた UI Automation のクライアントでウィンドウを操作する。

**Linux。** デスクトップがスクリーンリーダーの動作を通知している（`org.a11y.Status`）ときはアクセシビリティバスに接続し、そうでなければスクリーンリーダーの起動を待つ。`GOWAKU_A11Y=1` で常に接続し、`NO_AT_BRIDGE=1` で接続しない（GTK と同じ変数名）。セッションバスが無ければブリッジは働かない。ツリーは X11 と Wayland の両方で確認し、Orca のキー操作は Xvfb とヘッドレスの sway で試した。Wayland では画面上の座標が得られないので、ノードの位置はウィンドウ基準の座標になる。ライブリージョンは `Object:Announcement` で通知し、Orca 45 以降が読み上げる。

**macOS。** 各ノードは `NSAccessibilityElement` になる。テストでは要素とそのアクションに直接問い合わせている。VoiceOver での読み上げは、まだ手動では確認していない。

設計の背景: [ADR 0042](../../ADR/0042-accessibility-windows.md)（Windows、フォーカス、動きを減らす）、[ADR 0043](../../ADR/0043-accessibility-linux.md)（Linux）、[ADR 0085](../../ADR/0085-accessibility-macos.md)（macOS）。
