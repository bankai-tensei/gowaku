# 0136: 役割と操作の定数は semantics にだけ置く

決定日: 2026-11-16

状態: 採用

## 背景

`core` には、`semantics` の型と定数の別名があった。`core.SemanticsRole`、`core.SemanticsActions`、`core.SemanticsNode` と、役割の一部（`core.RoleButton`、`core.RoleCheckbox`、`core.RoleText` など 8 つ）と、操作の一部（`core.ActionTap` など 4 つ）である。

初めて使う人に gowaku を試してもらったところ、godoc の `core.RoleButton` を見て役割は `core` にあると考え、`core.RoleListItem` と書いてコンパイルエラーになった。リストの項目、見出し、タブなど、残りの役割は `semantics` にしか無い。別名が一部だけあるため、どこを探せばよいかが分からなくなっていた。

## 選択肢

- **A. 別名を消し、役割、操作、ノードは `semantics` だけに置く。** 書き方が 1 つになる。まだ公開前なので、互換のための別名は残さない。
- **B. 役割と操作をすべて `core` にも別名で置く。** どちらでも書けるようになるが、同じものに 2 つの名前が残り、godoc もどちらを使うかで揺れる。
- **C. 今のままにして、godoc に「残りは semantics にある」と書く。** 今もそう書いてあったが、読まれずにつまずいた。

## 決定

A を採る。`core.SemanticsRole`、`core.SemanticsActions`、`core.SemanticsNode`、`core.Role*`、`core.Action*` を消し、ライブラリ、テスト、ドキュメントの中の使い方を `semantics.Role`、`semantics.Actions`、`semantics.Node`、`semantics.Role*`、`semantics.Action*` に置き換えた。`core` 自身の型（`SemanticsBuilder`、`SemanticsArg`、`SemanticsActor`、`SemanticsBlocker`、`SemanticsIDOf`）はそのまま残す。これらは `semantics` の別名ではなく、木を作る側の型である。

## 結果

- 役割を探す場所は `semantics` の 1 つになった。
- 過去の ADR に出てくる `core.RoleButton` などは、書いた時点の記録として直していない。
