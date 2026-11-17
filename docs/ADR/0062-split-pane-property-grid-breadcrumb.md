# 0062: SplitPane、PropertyGrid、Breadcrumb（15b）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 §2 の 15b のうち、残りの 3 つ。どれもデスクトップのアプリの骨組みになる部品。

## 決定

### 1. SplitPane

- **ペインは 2 つだけ。** 3 つ以上は SplitPane を入れ子にする。
- **位置は部品の中に持ち、利用者の値から始める。**
  - ドラッグで動かした位置を部品が覚え、動くたびに `OnChange` で知らせる。
  - 利用者が新しい `Position`（または `Collapsed`）を渡したら、前の値と比べて置き換える。
  - こうした理由: 区切りの位置はアプリのデータではなく、配置の好みにあたる。すべての利用者に値を持たせると、ゼロ値で動かせなくなる。保存したい利用者は、受け取った値を渡し直せばよい。
- **単位。**
  - 既定は全体に対する割合。`Pixels: true` ならピクセルで、窓を広げても最初のペインの大きさが変わらない（サイドバー向き）。
  - 0 は `Default` を意味し、`Default` も 0 なら真ん中。ダブルクリックで `Default` に戻る。
- **畳む。**
  - Enter で `Collapsible` が指すペインを畳み、開くと前の大きさに戻る（ARIA の window splitter パターン）。
  - そのペインの最小の大きさの半分を越えて押し込んでも畳む。引き戻すと、最小の大きさで開く。
  - Home / End は限界まで動かすだけで、畳まない。
  - 畳んだペインは `core.Hider` で隠すので、ポインタ、Tab、アクセシビリティの木のどれからも外れる。
- **キーボード。** 区切りは Tab で止まる。動かせるのは軸に沿った矢印だけで、1 回 10px。ほかの矢印は外へ渡す。
- **見た目。**
  - 区切りは 1px の Hair の線で、当たり判定は 9px の帯。列や行の大きさを変えるカーソルを出す。
  - ホバー中とドラッグ中は Ink3。焦点があると AccentInk の線に淡い 5px の帯が付く（線に沿わせた焦点の輪）。
  - カーソルは新しく `input.CursorColResize` / `CursorRowResize` を足した（Win32、X11、Wayland）。DataTable の列の端でも使う。
- **Semantics。**
  - 区切りは `RoleSeparator` で、焦点を受け、最初のペインの割合を `Range`（0〜100）で持つ。Increase、Decrease、SetValue ができる。
  - 畳めるときは Expanded / Collapsed も持つ。UIA では RangeValue と ExpandCollapse になる。
  - 名前は既定で「ペインの区切り」。

### 2. PropertyGrid

- **型付きの編集欄で、リフレクションは使わない。**
  - 各プロパティは編集欄の値を持ち、そのコールバックは正しい型を受ける。ジェネリクスは選択肢の欄だけに使う。
  - CODING_STYLE §5.2 は、構造体のフィールドを reflect でたどることを禁じている。この形なら、利用者の型もコンパイラが確かめる。
  - 編集欄の種類: `PropertyText`、`PropertyNumber`、`PropertyInt`、`PropertyBool`、`PropertyChoice[T]`、`PropertyColor`、`PropertyCustom`。
- **各編集欄に、プロパティの名前を付ける**（ADR 0056 に残した「行の名前が欄の名前にならない」を、この部品では解いた）。
  - `core.SemanticsBuilder.Name(label)` を足した。`Describe` と同じく、次に開くノードにラベルが無ければそれを入れる。
  - 欄の横の名前の文字と、下の説明の場所は、アクセシビリティの木から外す。二重に読まれないようにするため。
  - 同じ仕組みで、あとで GroupRow も直せる。
- **分類。**
  - 分類は、その名前を持つ `RoleGroup`。見出しは Expanded / Collapsed を持つボタンで、Enter / Space で開閉し、← で畳み、→ で開く。
  - 開閉の状態は分類の名前ごとに覚え、`PropertyCategory.Collapsed` から始める。
- **列の境。** ドラッグでき、ダブルクリックで戻る。0 は「幅の 40%」で、名前に最低 80、値に最低 120 を取る。ホバー中とドラッグ中だけ見える。
- **絞り込み。** `Filter: true` で SearchField を足す。名前の部分一致で絞り、大文字と小文字を区別しない。分類の名前に一致したら、その分類を丸ごと残す。
- **説明の場所。** 下に高さ 80 で置く。説明を持つプロパティがあるときだけ出し、最後に焦点のあったプロパティの説明を見せる。
- **見た目。** 行は最低 34。名前は Ink2 で、焦点のある行は Fill の地に Ink の名前。焦点の輪のほかにアクセント色は使わない。

### 3. Breadcrumb

- **リンクはそれぞれ Tab で止まる**（ARIA の breadcrumb パターン）。
- **Semantics。**
  - 入れ物は `RoleGroup`「パンくずリスト」、リンクは `RoleLink`。
  - 今の場所は `RoleText` で、Hint「現在の場所」を付ける。`Value: "current"` にしなかったのは、ローカライズできて、ただの文字に Value パターンを生やさずに済むため。
- **溢れたとき。**
  - 最初と最後は残し、最初の次から順に「…」のボタン（「隠れている階層」）へ畳む。そのメニューから隠れた場所へ行ける。
  - 畳む判定は、Toolbar と同じくレイアウトの中でする。
- **見た目。**
  - 区切りは Ink3 のシェブロン。
  - リンクは Ink2 で、ホバーで Fill の形と手のカーソルが出る。今の場所は Ink。
  - 高さ 30。

## 結果

- golden は `pane`、`pane-dark`。
- gallery の「デスクトップ」が、この 3 つで組んである。
  - 左は区切りを畳めるフォルダの木、右はインスペクタ。上はパンくずリストで、木の選択に追従し、押すとその階層を選ぶ。
  - UIA では、区切り「フォルダの区切り」に RangeValue と ExpandCollapse があり、インスペクタの欄は「種類」のようにプロパティの名前で読めた。
- ベンチ（どれも 0 allocs/op）: SplitPane の Layout 173ns、Paint 371ns。Breadcrumb の Layout 132ns、Paint 395ns。
- 残り:
  - PropertyGrid の列の境は、キーボードと Semantics を持たない。
  - `PropertyNumber` は不正な文字をエラーで示さず、欄を離れると元の値に戻す。
  - `PropertyInt` の Stepper で、Tab が「−」に止まる。
  - Breadcrumb の「…」は開閉の状態を伝えない（Toolbar と同じ）。
  - 右から左の書字方向に対応していない。
  - SplitPane は、軸の向きに大きさが決まっていないと panic する。
  - Breadcrumb の長いラベルを「…」で切らない。
