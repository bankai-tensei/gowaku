# 0108: エディターの選択の通知と、レビューで見つかった 2 つの直し

決定日: 2026-11-13

状態: 採用

## 背景

- **要望 11:** キャレットが動いたことを知る手段が `editor.Editor` に無い。アプリは状態バーの「行・桁」を出すために 200ms ごとに `Selection()` を見に行っている。
- **レビュー:** ADR 0103 のビューで 2 つの不具合が見つかった。
  1. 一つの Controller を 2 つの Editor に出し、左の検索バーで `alpha`、右で `beta` を検索すると、文書の検索は `beta` になるのに、左のバーは `alpha` と古い件数を出したまま。左のバーで「すべて置換」を押すと、見えていない `beta` が置き換わる。
  2. View を Editor の間で動かすと（左右の View の入れ替え、キーの変更）、移動先の Editor が移動元より先に View を取った場合に、移動元の `detach` が持ち主の確認で早く返り、スクロール位置が保存されずに先頭へ戻る。

## 1. 選択の通知

### 選択肢

| | 内容 | 判断 |
|---|---|---|
| A. `Editor.OnSelectionChange` | Editor のフィールド。選択は ADR 0103 から View ごとなので、Editor が View を替えたときに誰の選択か曖昧になる | 採らない |
| B. `Controller.OnSelectionChange` | 主の View だけを知らせることになり、差分ビューアーの左右で別の状態バーを出せない | 採らない |
| C. `View.OnSelectionChange func(TextRange)` | 選択を持つ型に置く。`View.OnScroll` と並ぶ | **採る** |

名前はライブラリの既存の `OnSelectionChange`（`field.TextField`、`collection.DataTable` / `GridView` / `Tree`）に合わせた。`OnSelectionChanged` は使われていない。引数は `View.Selection()` と同じ `TextRange`（Start が anchor、End が caret）。

### 決定

- **呼ぶ時:** キー、ポインタ（クリック、ドラッグ）、編集、Undo / Redo、スクリーンリーダーの SetSelection、検索バーと F3、コードからの `Controller.SetSelection` / `View.SetSelection` / `FindNext` / `ReplaceMatch` など（その View に効くとき）。ほかの View での編集でこの View の選択が文字と一緒にずれたときも呼ぶ。前に知らせた値と同じなら呼ばない。
- **コードからの変更も知らせる。** `TextField.OnSelectionChange`（ADR 0075）がコントローラからの変更を知らせるのに合わせた。`OnScroll` の `SetScroll` が知らせないのは、左右のペインが互いに収め合う往復が終わらないことがあるからで、選択は丸めが無く、同じ値を返し合えば「前と同じ」で止まる。
- **呼ばない時:** `SetText`（`OnChange` も呼ばない。`TextField` に親が新しい `Value` を渡したときと同じ）と、Editor が View を出し始めたとき（build の中で起き、文書の選択を引き継いだり丸めたりするだけ）。どちらも、その時点の選択を「知らせた値」として覚える。
- **編集では `OnChange` の後に呼ぶ。** 行と桁を数えるときに本文が新しくなっている。`ReplaceMatch` は置換と次の一致の選択で 2 回呼ぶことがある。
- **付いていない View** の `SetSelection` も、値が変われば呼ぶ。
- **レイアウト中の変化は次のフレームの頭で知らせる（ADR 0103 の `OnScroll` と同じ決まり）。** 選択がレイアウトの中で変わるのは、フォーカスが移って IME の変換中の文字を確定するときで、検索バーの `focusQuery` はレイアウトの中でフォーカスを取る。そこで `FocusChanged(false)` の間は Controller を「静か」（`quiet`）にし、その間の変化は各 View の描画オブジェクトの Ticker（`OnScroll` と共用の `notify`）で次のフレームに知らせる。ほかの経路（入力、コールバック）はその場で知らせる。
- **アロケーション:** 比較は `selection` 2 つの比較だけ。コールバックがあっても、`SetSelection` + レイアウト + 描画は 0 アロケーション（`TestView_OnSelectionChangeAllocatesNothing`）。

## 2. 検索バーは文書の検索を映す

- ADR 0103 の「検索は文書に 1 つ」は変えない。Controller が「バーに見せる検索」（打ったままの文字、オプション、正規表現の誤り）を持ち、`Find` と検索の終わりで更新する。`findChanged` で、開いている全部のバーの `query` / `options` / `badQuery` をそれに合わせて作り直す。だから、どのバーも見せていない検索で置換することは無い。
- どれかのバーを閉じると検索が終わり（ADR 0103 どおり）、ほかの開いているバーは空の検索を見せる（件数も出ない。押しても何も置き換えない）。閉じたバーは自分の文字を覚えていて、次の Ctrl+F でそれを検索し直し、ほかのバーもそれを映す。
  - 案として「1 つ閉じたら全部閉じる」も考えたが、触っていないペインのバーが消えるのは驚きが大きい。

## 3. 動いた View は位置を持っていく

- 描画オブジェクトが View を取るとき、まだ別の描画オブジェクトがその View を持っていれば、先にそちらのスクロール位置を View の `pending` に移す（`keepPlace`）。選択は View にあるので何もしなくてよい。
- 取られた側がそのまま木に残れば、今までどおり次のレイアウトか描画で panic する（`TestView_ShownTwicePanics`）。入れ替えでは、取られた側も同じ build の中で別の View を取るので panic しない。

## 結果

- テスト: `editor_review_test.go`（2 つのバーが同じ検索を見せる、すべて置換は見えている文字を置き換える、誤りとオプションも映る、閉じたあと / 開き直したあと。View の入れ替え、キーの変わった新しい Editor へ移る）、`editor_selnotify_test.go`（上の「呼ぶ時」「呼ばない時」、2 つの View、レイアウト中は次のフレーム、0 アロケーション）。どれも直す前に失敗することを確かめた。
- 公開 API は `View.OnSelectionChange func(TextRange)` だけを足した。
