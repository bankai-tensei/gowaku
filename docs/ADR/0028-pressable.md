# 0028: 押せる領域 Pressable を部品として持つ

決定日: 2026-11-08

状態: 採用

## 背景

公開 API だけで音楽プレーヤーの見本アプリを書いたところ、自前のボタンのような物を作るたびに、状態を持つ包みを手で書くことになった。見本アプリ の `hover`（`ui.go`）がそれで、次の 3 つを重ねて `Build func(on bool) Widget` を呼ぶ。

- `MouseRegion`: `OnEnter` / `OnExit` と手の形のカーソル
- `GestureDetector`: `OnTap`
- `Semantics`: `RoleButton` と `Label`

使い道は、アイコンだけの丸いボタン、再生ボタン、アルバムのタイル、曲の行、「もっと見る」のリンクと、画面のほぼ全部に及んだ。ポインタが乗ったら色を変える押せる領域は、どのアプリにも要る。

一方で `hover` には足りないところがある。

- 押している間の状態が取れない。
- Tab で辿れず、Enter や Space で押せない。
- `OnTap` を外したときに、子の State が作り直される。包みの形が変わるため。

## 選択肢と決定

### 1. どこに置くか

- (a) `Button` に `Build` を足す。
  - `Button` は見た目（地の色、枠、影、沈み、焦点の輪）を持つ部品で、そこに「見た目を持たない」モードが混ざる。
- (b) 見た目を持たない部品を別に作る。`Button` は今のまま。

**(b) を採る。** 名前は `Pressable`（React Native と Jetpack Compose と同じ語）。押されたら走る関数は `Button` に揃えて `OnPress` とした。キー操作でも走るので `OnTap` より合う。

### 2. ビルダーに渡す状態の形

- (a) `func(hovered, pressed bool)` のように bool を並べる。増やすたびに全員の関数の形が変わる。
- (b) ビットの集合。`s&PressHovered != 0` は読みにくい。
- (c) bool を持つ小さな構造体 `PressState{Hovered, Pressed, Focused, Enabled}`。

**(c) を採る。** `s.Hovered` と読め、項目を足しても既存のコードは壊れない。ゼロ値は「無効で、何も触れていない」で、意味のある既定になる。

### 3. 何を状態として持つか

- `Hovered`: ポインタが上にある。
- `Pressed`: 主ボタンが下りてから上がるまで。外へドラッグしても上がるまでは true（`RenderGesture` の押下と同じ区間）。
- `Focused`: キーボードを持っている。`Button` と同じく `Focus` を使い、Tab で辿れ、Enter と Space で `OnPress` が走る。キーの押しっぱなしでは繰り返さない。クリックでは焦点は移らない（`Tree` の既存の挙動）。
- `Enabled`: `OnPress` があるか。

`OnPress` が nil なら無効とし、カーソルは矢印、焦点に入らず、ほかの 3 つは常に false で渡す。

### 4. 無効のときの木の形

- (a) 無効なら包みを外し、子だけを返す（`control.wrap` のやり方）。
  - 有効と無効が切り替わると、子の State（`AnimatedContainer` の途中の値など）が捨てられる。
- (b) 包みは常に置き、無効のときは中身を空にする。
  - `Focus{Enabled: false}` にする。
  - `GestureDetector` のコールバックは nil にする。
  - `MouseRegion` のカーソルは矢印にする。

**(b) を採る。** `MouseRegion` は無効の間もポインタを追い続ける。ポインタの下で有効に戻った領域が、すぐに `Hovered` になるようにするため。押している途中で無効になったときは、`RenderGesture` が離したことを伝えないので、`DidUpdateWidget` で `pressed` を下ろす。

### 5. 見た目と動き

`Pressable` 自身は何も描かず、アニメーションも持たない。フラグはその場で切り替わる。フェードやスプリングは、ビルダーが `AnimatedContainer` などに任せる。`control` のバネや色の補間を持たせると、使わない人にも Ticker が付いてくるため。

焦点の輪も描かない。形を知らないからで、`Focused` を見て描くよう doc に書いた。

### 6. Semantics

常に `Semantics` で包む。`Role` のゼロ値は `RoleButton` にする。`Value` も渡せるので、切り替えボタンを `RoleCheckbox` と `"checked"` で表せる。`Label` が空でもボタンの node は作り、中の文字はその子の node として並ぶ。

### 7. Button を Pressable で書き直すか

**今は書き直さない。** `Button` / `Checkbox` / `Switch` は `control` を共有し、ホバーの補間、押下のバネ、キーで押したときの一蹴りを持つ。`Pressable` の上に載せるには、それらを State ごと移すことになり、見た目の golden にも響く。危険のわりに得るものが少ない。

## 影響

- `widgets.Pressable` と `widgets.PressState` を足した。ルートの `gowaku` からも同じ名前で使える。
- 包みのコールバックは `InitState` で一度だけ作る。Build ではクロージャを作らない。
- 見本アプリ の `hover` は `Pressable` に置き換えられる。`func(on bool)` は `func(s PressState)` になり、`on` は `s.Hovered` になる。
- 押せる領域が Tab の止まり先になる。リストの行をすべて `Pressable` にすると Tab の歩数が増えるが、これは `Button` を並べたときと同じ。
