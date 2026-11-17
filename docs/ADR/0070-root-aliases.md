# 0070: ルートの別名を全部のキーと geom のコンストラクタへ広げる

決定日: 2026-11-09

状態: ADR 0081 で置き換え

## 背景

サンプルアプリを書いたところ、`gowaku` だけを import するつもりが `geom` と `input` も要った。

- メニューのショートカット（Ctrl+N、Ctrl+1）に要る `KeyN`、`Key1`、F3 などがルートに無かった。gowaku.go は「よく使うキー」だけを置いていた。
- `Hex` はあるが `RGBA` が無く、半透明の色に `geom` が要った。
- `MouseRegion.Cursor` の `input.Cursor`、`PointerEvent.Button` の `input.Button` にもルートの名前が無かった。
- `Transform` の `Affine` は別名があるのに、作る関数（`geom.Rotation` など）が無かった。
- `RRectAll`、`All`、`Symmetric` はすでにある（著者が `geom.RRect` に Radius が無いことで詰まったのは、ルートの `RRectAll` を見落としたため）。

## 決定

- **キーと修飾キーは全部** ルートに置く。`input` の `Key*` と `Mod*` を同じ名前で。新しいファイル `aliases_input.go` に足し、gowaku.go の既存の別名は動かさない（ADR 0016、追記のみ）。他のエージェントと衝突しないよう、別名は新しいファイルに置く。
- ポインタのボタンは `PointerButton` 型と `PointerPrimary`、`PointerSecondary`、`PointerMiddle`、`PointerBack`、`PointerForward`。`ButtonPrimary` はすでに Button の見た目の名前なので、`Pointer` を前に付けた。
- カーソルは `Cursor` 型と `CursorArrow` ほか 5 つ。`input` と同じ名前。
- geom のコンストラクタは `aliases_geom.go` に、すでに別名のある型を作るものだけを置いた: `RGBA`、`Identity`、`Translation`、`Scaling`、`Rotation`、`Skewing`。`Constraints` や数学の関数は UI のファイルに要らないので置かない。
- **同期のテスト** `TestRootKeysMatchInput`（aliases_test.go）: go/parser で `input/` とルートの非テストのファイルを読み（ビルドタグに関係なく）、
  - `input` の `Key*`、`Mod*`、`Cursor*` はルートに同じ名前で、`Button*` は `Pointer*` で、値が `input.<同じ名前>` であること、
  - 逆に、ルートで `input.X` から取った定数が X の名前を持つこと（`KeyF3 = input.KeyF4` を防ぐ）、
  を確かめる。`input` にキーを足すと、別名を足すまでこのテストが落ちる。

## 結果

- 足した名前: キー 53、型 2、ボタン 5、カーソル 5、関数 6。キーと修飾キーは `input` の 74 個すべてがルートにある。
- ドットインポートするファイルでは名前が増えた分、アプリの名前（`Identity`、`Rotation`、`Cursor`、`KeyA` など）と衝突しうる。doc.go に書いたとおり、どちらかの名前を変える。
