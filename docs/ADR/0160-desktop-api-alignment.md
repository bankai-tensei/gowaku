# 0160: デスクトップの API を ctx 経由にそろえ、platform の型を公開 API から外す

決定日: 2026-11-20

状態: 採用（ADR 0022、0049、0067、0081、0143 の該当部分を置き換える）

## 背景

v0.1.0 の外部レビューで、v1.0 の前に直したい公開 API の不ぞろいが見つかった。

- `gowaku.FileDialog`、`FileFilter`、`ClipboardContent`、`ClipboardKinds` が `platform` の型の別名（`=`）になっている。`platform` は互換性を約束しないパッケージなので（ADR 0016）、その型が変わると `gowaku` の API まで変わってしまう。
- `ShowTray`、`Notify`、クリップボードの関数は `ctx.Host()` を通るので、`gowakutest.Tester` の下でも動く。一方、`ShowFileDialog`、`SetTheme`、`CurrentTheme` はグローバル変数の `currentApp` を読むため、Tester の下では panic する。
- `field.Input.Extras` で外から設定できるのは `OnFocus` だけで、ほかは非公開の部品だった。
- ウィンドウの `OnCloseRequest` は `func() bool` で、閉じてよいかを返す。ところが `Dialog` と `Sheet` の `OnDismissRequest` は `func()` で、閉じる処理そのものを置き換える。
- `ShowToast` は閉じる関数を返すのに、`ShowDialog` と `ShowSheet` は返さない。

v1.0 までは、マイナーバージョンで公開 API を変えてよい。互換のための別名は残さず、v0.2.0 でまとめて変える。

## 決定

### 1. 型の本体を `core` に置く

`ClipboardKinds`、`ClipboardContent`、`FileDialog`、`FileFilter` は `core` で定義する。`gowaku` では `Widget` や `BuildContext` と同じように別名で見せる。フィールドは前と同じなので、`gowaku.FileDialog{...}` と書いたコードはそのまま通る。`platform` は自分の型を持ち続け、`gowaku` が境界で変換する。

`gowaku` に本体を置く案もあった。しかしそうすると、`gowakutest` が `gowaku` を import しなければならない。`gowaku` は `internal/inspect` などを import しているので、それらのパッケージ内テストが `gowakutest` を使えなくなる（import の循環になる）。`gowaku` より下のパッケージが自分のテストで `gowakutest` を使えることは手放したくないので、`core` に置いた。

`ThemeMode` も同じ理由で `theme.Mode` に移し、`gowaku.ThemeMode` はその別名にした。`theme.Mode.For(os)` は OS の明るさを受け取り、使う明るさを返す。

### 2. `ShowFileDialog`、`SetTheme`、`CurrentTheme` は ctx を取る

```go
func ShowFileDialog(ctx BuildContext, d FileDialog, done func(paths []string, err error))
func SetTheme(ctx BuildContext, set theme.Set, mode ThemeMode)
func CurrentTheme(ctx BuildContext) (theme.Set, ThemeMode)
```

- ファイルダイアログは `core.FileDialogHost` を通る。Run のウィンドウと Tester がこれを満たす。ダイアログは、フォーカスのあるウィンドウではなく `ctx` のウィンドウの手前に出る。トレイのメニューやタイマーの ctx などでは、そのウィンドウがすでに閉じていることもある。その場合は開いている別のウィンドウ（フォーカスのあるものを優先）の手前に出し、開いているウィンドウがなければ `ErrNoFileDialog` を返す。
- テーマは `gowaku` 内の非公開の interface を通る。Tester はメソッドの形を合わせてこれを満たすので、`gowaku` を import せずに済む。
- どちらも満たさないホストでも panic しない。`ShowFileDialog` は `ErrNoFileDialog` を渡して `done` を呼び、`SetTheme` は何もせず、`CurrentTheme` はゼロ値の `theme.Set`（`App` と同じく `theme.Default()` を表す）と `ThemeSystem` を返す。
- `WindowHandle.ShowFileDialog` は残す。ウィンドウを持っていてもその `ctx` を持たないコードは、こちらを使う。

Tester には偽のファイルダイアログを足した。`tt.FileDialog()` で表示中のダイアログを取り、`Choose(paths...)`、`Cancel()`、`Fail(err)` で答える。`SetTheme` は Tester のツリーのテーマを切り替え、`ThemeSystem` のときは `Options.Dark` を OS の明るさとして扱う。

### 3. `field.Input.OnFocus`

`OnFocus func(focused bool)` を `field.Input` に直接置き、`Extras` 型とそのフィールドは非公開にした。

### 4. `OnDismissRequest func() bool`

`Dialog` と `Sheet` の `OnDismissRequest` は、Escape や外側のクリックで閉じるよう求められたときに呼ばれる。true を返せば閉じ、false を返せば開いたままになる。nil なら閉じる。ウィンドウの `OnCloseRequest` と同じ形になる。`build` が受け取る `dismiss` 関数は、`OnDismissRequest` に尋ねずに閉じる。

### 5. `ShowDialog` と `ShowSheet` は `dismiss` を返す

`ShowToast` と同じく閉じる関数を返す。`build` が受け取る `dismiss` と同じ関数で、閉じかけているときや閉じた後に呼んでも何も起きない。最初の描画より前に呼べば、ダイアログは表示されない。

## 影響

- v0.2.0 で破壊的変更になる。移行の方法は CHANGELOG の Changed に書いた。
- `gowakutest` の `ClipboardContent` と `SetClipboardContent` は `core.ClipboardContent`（`gowaku.ClipboardContent` と同じ型）を使う。
- `TrayOptions` から作る `platform.TrayOptions` は、まだ `gowakutest.Tray.Options()` で見えている。今回は扱わず、次に直す。
