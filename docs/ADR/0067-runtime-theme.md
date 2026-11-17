# 0067: 実行中のテーマ切り替えと、呼び出し元のテーマに従うオーバーレイ

決定日: 2026-11-09

状態: 採用

## 背景

サンプルアプリの設定ページで、アクセント色とライト/ダークを実行中に変えたかった。困ったことが 2 つあった。

- `App.Theme` と `App.ThemeMode` は起動時に固定で、変える手段が無かった。アプリは自分の中身を `widgets.Theme{Data: th, Child: widgets.Overlay{Child: ...}}` で包み、MediaQuery の明るさまで上書きしていた。
- ルート（root.go）は `MediaQuery → Localizations → Theme → Overlay → Home` の順に包む。`ShowDialog` / `ShowToast` / `ShowSheet` はいちばん近い Overlay に差し込むので、アプリが自分の Overlay を忘れると、ルートの Overlay、つまりアプリの Theme より上に出る。ダイアログ、トースト、シートが古いテーマで出た。メニュー、ツールチップ、ドロップダウン、候補の一覧も同じだった（DatePicker だけは自前でテーマを運んでいた）。

## 選択肢

### 1. 切り替えの API の形

- **A. ウィンドウごと: `gowaku.WindowOf(ctx).SetTheme(set, mode)`。** 窓ごとに違うテーマを持てる。ただし、窓ごとに OS の明るさを追う状態が要る。`OpenWindow`、`ShowModal`、`TearOffWindow` で開く窓がどのテーマを継ぐかも決めなければならない。
- **B. アプリ全体: `gowaku.SetTheme(set, mode)`。** 今のテーマはもともとアプリの状態（`app.themes` / `app.mode`）で、OS の明るさの変化も全部の窓に配っている。`App.Theme` を実行中に変えるだけ、と説明できる。
- **C. 何もしない。** アプリが `widgets.Theme` で包む今のやり方のまま、オーバーレイだけ直す。ただ、タイトルバーの暗さとウィンドウの背景は `widgets.Theme` では変わらない。

### 2. オーバーレイが呼び出し元のテーマを得る方法

- **a. 呼んだ時点の値を写す。** 簡単。ただし、開いているダイアログやトーストが `SetTheme` や OS の切り替えに付いてこない。テーマを選ぶダイアログの中で色を変える、という素直な使い方で古いまま残る。
- **b. 呼び出し元の位置を覚え、その上の値に依存し続ける。** 開いている間も追従する。そのため core に「別の位置から見た Inherited に依存する」手段が要る。
- **c. 呼び出し元が自分の Overlay を置く決まりにする。** 今の不具合そのものなので却下。

## 決定

### 1. `gowaku.SetTheme(set, mode)` と `gowaku.CurrentTheme()`（B）

- 新しいファイル `theme_runtime.go` に置いた（`gowaku.go` の別名の追加と衝突しないように）。
- テーマはアプリ全体のものにした。理由は 3 つ。
  - 今もアプリの状態だから。
  - 設定ページの「アクセント色」「ダークモード」は、アプリの全部の窓に効くのが普通だから。
  - 後から開く窓、モーダルの窓、ちぎったタブの窓が、何も考えなくても同じ見た目になるから。
- 窓 1 つ、あるいは木の一部だけ違う見た目にしたいときは `widgets.Theme` で包めばよい。決定 2 でオーバーレイもそれに従うようになったので、これで足りる。
- `SetTheme` がすること。
  - set と mode を差し替え、OS の明るさを読み直す。
  - 全部の窓に `applyTheme`（背景とタイトルバーの暗さ）と `rootState.refresh` をかける。ルートの `widgets.Theme` の値が変わり、`ThemeOf` に依存する部品だけが組み直される。
  - 同じ set と mode なら何もしない（`theme.Set` は 2 つのポインタなので比べられる）。
- `ThemeSystem` に戻すと、その時点の OS の明るさから始め、以後も `EventBrightness` に従う（`followOS` は mode によらず明るさを覚え続けている）。
- Windows（`run_windows.go`）も Linux（`run_linux.go`）も同じ `run` を通るので、プラットフォームごとの分岐は無い。偽のプラットフォームのテストでそのまま確かめられる。
- UI ゴルーチン専用で、`Run` の外では panic、Build の中では SetState の契約どおり panic する。
- **`MediaQueryData.Brightness` は変えない。** v1 の契約が「テーマによらず OS が求める明るさ」だからで、起動時の `ThemeMode` でも同じ。使っている明るさは `ThemeOf(ctx).Brightness` で読む。アプリが MediaQuery を上書きしていたのは自前の Theme の包みに合わせるためで、`SetTheme` を使えば要らない。
- `CurrentTheme()` は設定ページが今の状態を表示するための読み出し。

### 2. オーバーレイは呼び出し元の位置に従う（b）

- **core** に `DependOnAt[T](ctx, at BuildContext) (T, bool)` を足した（v1 への追加のみ）。at の上でいちばん近い `Inherited[T]` を返し、ctx をその依存先に登録する。
  - at が木から外れていても、要素の親へのポインタは残っている。だから外れた時点の祖先をたどれる。
  - そのうち木に残っている祖先（たとえばルートの Theme）が変われば追従する。外れた祖先は二度と変わらないので、最後の値のまま。
  - 削除した行から出したトーストが、行が消えても `SetTheme` には付いてくる、というふるまいになる。
- **widgets** に `OverlayState.InsertFrom(ctx, build)` を足した。
  - 中身を非公開の `callerScope` で包む。
  - `callerScope` は ctx の上の `*theme.Theme`、`MediaQueryData`、`*localized`（Localizations）に依存し、それを `Inherited` として子に渡す。
  - ctx の上に無いものはオーバーレイの位置の値で埋める。こうして常に同じ 3 段を置き、値が変わっても下の State を作り直さないようにした。
  - Localizations が上に無いときは nil の `*localized` を置くことになるので、`StringsOf` などの読み出し側で nil を「無し」と扱うようにした。
- 運ぶものを 3 つに絞った理由。
  - テーマの拡張（ADR 0065）は `*theme.Theme` の中にあるので、テーマと一緒に運ばれる。
  - フォームや欄の範囲（`formState`、`fieldScope`）は、オーバーレイに運ぶと意味が変わるので運ばない。
  - `*text.Shaper` と `*window` は窓に 1 つで、オーバーレイの位置でも同じ。
- **`Insert` はそのまま**（オーバーレイの位置の値を使う）。ちぎれない共有の層、たとえばトーストの台はこちらを使う。
- `InsertFrom` に切り替えたもの: `ShowDialog`、`ShowSheet`、メニュー（`ShowPopupMenu`、MenuBar、ContextMenu が通る `openMenu`）、Tooltip、DropdownMenu、Autocomplete、DatePicker。
  - DatePicker は前から自分で Theme / MediaQuery / Localizations を運んでいて、それは残した。`InsertFrom` で、開いている間の変化にも組み直されるようになった。
- **トースト**は 1 つのオーバーレイに 1 つの台（エントリ）を共有する。
  - 台は `Insert` のまま、`toastItem` に呼び出し元の ctx を持たせ、カード 1 枚ずつを `callerScope` で包んだ。
  - 違うテーマの場所から出したトーストが同じ列に並んでも、それぞれ自分のテーマで出る。
- **いつの値か**: 呼んだ時点で写すのではなく、ずっと追う。
  - 表示中も呼び出し元の位置の値を追う（`SetTheme`、OS の明るさ、部分の `widgets.Theme` の差し替えのどれでも組み直される）。
  - 呼び出し元が木から外れたあとは、外れた時点の祖先の値。そのうち木に残っている部分は追い続ける。
- 費用: エントリごとに祖先をたどるのが 3 回と、`Inherited` が 3 段。オーバーレイの組み直しはホットパスではない。

## 結果

- 設定ページは `gowaku.SetTheme(gowaku.ThemeFromAccent(c), gowaku.ThemeDark)` を呼ぶだけでよい。自前の `widgets.Theme` と Overlay と MediaQuery の上書きは要らない。
- 部分だけ見た目を変える `widgets.Theme` の包みも、ダイアログ、トースト、シート、メニュー、ツールチップ、ドロップダウンまで効く。Overlay を自分で置く必要は無い。
- テスト（GPU 無し）:
  - core: 別の枝からの依存と、見つからないとき。
  - widgets: 入れ子の Theme の下から開いたダイアログ（言語も含む）と、その Theme の差し替えへの追従。呼び出し元が消えたあと。トーストが 1 枚ずつ呼び出し元のテーマを取ること。`Insert` が変わらないこと。
  - ルート: `SetTheme` が全部の窓を組み直し、タイトルバーと背景を変え、`ThemeSystem` で OS に戻ること。開いているダイアログに届くこと。
- 残り:
  - DataTable のフィルタのポップアップ（`widgets/datatable_filter.go`）はまだ `Insert` を使う。別の作業と並行しているので触っていない。`InsertFrom(c.base.Context(), ...)` への 1 行の置き換えで揃う。
  - gowakutest に `SetTheme` 相当は無い。`Options.Theme` で作り直すか、`widgets.Theme` で包んで試す。
  - テーマの切り替えを補間でなめらかにすること（ADR 0065 の `Lerp` と同じく、必要になってから）。
