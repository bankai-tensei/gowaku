# 0075: TextFieldController と枠の無い TextField

決定日: 2026-11-09

状態: 採用

## 背景

notes サンプルを作ったところ、TextField に 2 つ足りないものがあった。

1. **外から選択とフォーカスを扱えない。** サンプルは代理の RenderObject で内部の `RenderEditable` を探し、Semantics のアクションやキーの合成で操作していた。行と桁は 120ms ごとのポーリングで取っていた。`Selection()` が端を整列して返すので、後ろ向きの選択ではキャレットの位置が分からなかった。
2. **画面全体のエディタでも入力欄の枠が付く。** 地の色、枠、フォーカスリングが描かれ、メモ帳全体が 1 つの大きな入力欄に見えた。

ヒアリングで、どちらもライブラリで直すことにした。

## 決定

### 1. TextFieldController

- `TextField.Controller *TextFieldController` と `TextField.OnSelectionChange func(anchor, caret int)` を足した。コントローラはゼロ値で使える。
- メソッド: `Text`、`Selection() (anchor, caret)`、`SetSelection`、`SelectAll`、`SelectedText`、`Focus`、`Focused`、`Replace(start, end, s)`、`Insert`、`CanUndo` / `CanRedo` / `Undo` / `Redo`、`Copy` / `Cut` / `Paste`。
- **本文は `Value` に置いたまま。** コントローラは render object に届く窓口で、本文を持たない。
  - TextEditorController（ADR 0061）は本文を持つが、TextField がそうすると、ゼロ値で使えて Value に従うという今の約束が壊れる。
- **オフセット**: 書かれたままのテキストのバイト位置を使う。ルビの記法を含み、IME の未確定文字列は含まない。OnChange / Value と同じ空間である。範囲外は丸め、文字の途中は文字の先頭に戻す。
- **`Selection()` は (anchor, caret)。** キャレットは動く側の端なので、後ろ向きの選択も向きが残る。TextEditorController（Start = anchor、End = caret）と揃えた。`RenderEditable.Selection()`（整列した端）はそのまま残した。
- **表示していない間も選択を覚える。** 同じコントローラで作り直したフィールドは、その選択を引き継ぐ。notes は文書ごとにコントローラを 1 つ持つ。だから、タブを戻したときにキャレットが戻る。
  - フィールドが無い間、編集のメソッドは何もしない。
  - Build の中から呼んでよいのは、まだフィールドに渡していないコントローラへの SetSelection と Focus だけ。他は TreeController と同じく、コールバックと InitState から呼ぶ。コールバックを走らせるためである。
- **OnSelectionChange は 1 つの経路で呼ぶ。** キー、ポインタ、IME の確定、Undo / Redo、スクリーンリーダーの SetSelection、コントローラ、のどれでも同じ経路を通る。
  - 編集のときは OnChange の後に呼ぶ。親が行と桁を数えるときに、本文が新しくなっている。
  - IME の未確定中はキャレットが値の中で動かないので、呼ばない。
  - コールバックが無ければ、増えるのは int 2 つの比較だけで、アロケーションしない。
- **親が新しい Value を渡したとき**は、今までどおり本文を置き換え、履歴を消す。選択は丸めて残し、OnSelectionChange は呼ばない。Build の中で起き、変えたのは親自身だからである。
- **編集の呼び出し**は、どれも最初に IME の未確定文字列を確定する。ポインタを押したときと同じ扱い。
  - `Replace` は閉じた 1 つの Undo の単位になる。
  - 選択の動きは TextEditorController.Replace と同じ: 範囲より前なら留まる、後ろならずれる、重なれば新しい文字列の後ろのキャレットになる。
  - 単一行のフィールドでは、改行を空白に変える。
  - 「すべて置換」は、最初の一致から最後の一致までを 1 回の `Replace` にすれば、Undo 1 回で戻る。複数範囲や ReplaceAll の API は作らなかった。
- **ルビ（ADR 0074）**: SetSelection は矢印キーと同じ経路を通る。だから端をマークの中に置くと、そのマークが開く。
- **共有**: 最後に作られたフィールドがコントローラを取る。TextEditor と同じ。

### 2. 枠の無い TextField: `Bare bool` と `Padding`

- `Bare` は、地の色、枠、フォーカスリング、ホバーを描かない。クリップはフィールドの境界で行う（今までは内側に 2px）。
- フォーカスは点滅するキャレットだけで示す。メモ帳、テキストエディット、Word と同じである。
  - フォームのエラーはメッセージでだけ示す。Semantics は変えない。
  - Bare でも薄いリングを残す案は採らなかった。画面全体のエディタの見た目が崩れる。
- `TextEditor.Border bool` に揃えて bool にした。変種の enum（FieldStyle）は作らなかった。ButtonStyle が enum なのは変種が 4 つあるからである。
- `Padding geom.Insets`: ゼロなら、枠付きは今までの内側の余白、Bare は余白無し。それ以外の値は余白を置き換える。
  - 外側の Padding ではなく内側に持つので、スクロールした本文が余白の下まで流れる。余白の所で切れない。
- golden は `field-bare` と `field-bare-dark`。

## 残したこと

- 複数範囲の選択（矩形選択）は無い。
- notes の行と桁は、OnSelectionChange を受けてアプリ側で数えている。TextField に行と桁を返す API は足していない。
