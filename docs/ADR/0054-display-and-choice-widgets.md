# 0054: 表示と選択の部品（15a）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 の 15a のうち、Divider、Badge、Card、Chip、Toolbar、Radio / RadioGroup、Tabs。どれも [DESIGN.md](../DESIGN.ja.md) の言語で作る。

## 決定

### 1. 表示

- **Divider**: `Hair` の 1px。ゼロ値は横線で幅いっぱいに伸び、`Axis: layout.Vertical` で縦線になる。`RoleSeparator`。
- **Badge**: 表示の優先は Label、Count（0 より大きいとき）、`Dot: true` の点の順で、どれも無ければ何も出さない。**ゼロ値は子だけを描く**ので、件数が 0 になると if を書かなくても消える。`Child` の右上にかかって載り、大きさを持たない。左端を固定し、長い件数は外へ伸びる。塗りは Accent に白文字、`Quiet` は Fill に Ink2。
- **Card**: 地は Card、角丸 12、Hair の線、LiftControl の影、余白 16。`OnPress` があるとカード全体がボタンになる。**押したときの沈みは 0.985 にした**（DESIGN §7 は 0.97）。カードは幅が広く、0.97 だと縁が数 px 動いてしまう。**レビューで確認する。**
- **Chip**: 高さ 24 の丸い形で、3 種類ある。`OnChange` は切り替え（`RoleCheckbox`、選択中は AccentTint）、`OnPress` は押すもの（`RoleButton`）、どちらも無ければただのタグ。`OnRemove` は末尾に × を出し、フォーカス中の Delete / Backspace でも消える。何も付けないチップを「無効」にしないのは、よく使う「× 付きのタグ」が無効に見えてしまうため。

### 2. Toolbar

- 項目は `Children` ではなく `Items []ToolbarItem` で受ける。入りきらない項目をメニューに出すには、ラベルと処理が要るため。ボタンは Phase 14 の `Action`（ADR 0046）で表し、メニューとショートカットに共有できる。自前の部品は `ToolbarItem.Child` で入れる。
- Tab で止まる場所は 1 つで、矢印、Home、End で移る（Windows と ARIA の toolbar と同じ）。
- 溢れの判定はレイアウト中にする。隠す項目を `core.Hider` にすると、ヒットテスト、Tab、Semantics から自動で外れる。隠れた項目は末尾の「…」から、既存のメニュー（`openMenu`）で出す。
- 高さ 38（中のボタンは 30）。Checkable な Action は `RoleCheckbox` になる。

### 3. Radio と Tabs

- **RadioGroup** は値の型をジェネリクスで受ける（`RadioGroup[T comparable]`）。Tab で止まる場所は 1 つで、選択中の項目に入る。矢印は移動と同時に選択する。グループは `RoleRadioGroup`、項目は `RoleRadio` で `Selected`。単独の `Radio` も使える。
- **Tabs の印は板にした（下線にしない）。** DESIGN では、選択を線ではなく面で示す（Sidebar の AccentTint、Segmented の板）。下線はやめた MD3 の署名にあたる。選択中のタブの後ろに `Fill` の板（角丸 8、高さ 30）を置き、Segmented と同じスプリングで滑らせる。Segmented と見分けるため、トラックは持たず、タブは文字の自然な幅で左に寄せ、列の下に髪の毛の線を引く。アクセント色は使わない。
- タブの列は Tab の 1 停止で、矢印で移ると同時に開く（自動で開く方式）。Ctrl+Tab と Ctrl+PageDown は、Tabs の中のどこにフォーカスがあっても効く。中身は AnimatedSwitcher でクロスフェードする。ReduceMotion では板の移動をやめ、フェードは残す（ADR 0042）。
- 閉じる、並べ替える、引き剥がす `TabBar` は 15b の別の部品にする。項目の型を `TabPage` にして、名前がぶつからないようにした。

### 4. 選択肢の型の `Disabled`

`RadioOption.Disabled` と `DropdownOption.Disabled`（ADR 0056）は、CODING_STYLE §4.1 の「bool は肯定形」の例外にした。`Enabled bool` にすると、ゼロ値の `{Value, Label}` が無効になってしまう。選択肢はほとんどが有効なので、書き忘れると全部が無効になる罠を避けた。部品そのもの（`Radio`、`DropdownMenu`）は、今までどおり `OnChange` が nil のときに無効になる。

### 5. ボタンとリンクは中の文字を名前にする（core）

UIA で確かめたところ、`Label` の無い押せるカードが、**名前の無いボタン**になっていた。中の文字は、ボタンの横に別のテキストとして並んでいた。ARIA の name from content に合わせて、`RoleButton` と `RoleLink` に Label が無ければ、中のテキストを空白でつないで名前にし、そのテキストは子から外すことにした（二重に読まれないように）。`core.SemanticsBuilder.Close` で行う。

## 結果

- golden は `display`、`choice`（ライトとダーク）。gallery の「表示」「フォーム」のページに並べた。
- Toolbar、Tabs、RadioGroup は、gallery を UIA から操作して確かめた。名前、状態、無効、選択の切替が読める。
- 残り: 個別のタブの無効化と、ページの State の保持（keep-alive）は作っていない。Toolbar の「…」はメニューの開閉を Semantics で伝えていない。右から左の書字方向の矢印キーは考えていない。
