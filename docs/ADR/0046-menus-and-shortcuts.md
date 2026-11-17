# 0046: メニューとショートカット（14a）

決定日: 2026-11-09

状態: 採用

## 背景

OS 統合（Phase 14）を 14a と 14b に分けた。14a はモーダル、メニュー + ショートカット、カスタムタイトルバー、クリップボード拡張。この ADR はメニューとショートカット。完了条件は「メニューがキーボードだけで操作できる」。当初の計画は「描画は全 OS で自前、macOS のメニューバーだけ `NSMenu` に写す」としている。

ヒアリングで次を決めた。

- **`MenuBar` は利用者が置く普通のウィジェット。** Column の先頭に置けば従来の位置、タイトルバーの中に置けば VS Code 風。macOS では隠れて `NSMenu` になる。
- **メニューとショートカットは `Action` を共有する。** ラベル、キー、今使えるかを 1 か所で言う。

## 選択肢

1. Flutter の `Shortcuts` / `Actions` / `Intent` を写す。キーから Intent、Intent から Action を引く 2 段。
2. `MenuItem{Label, Shortcut, OnSelect}` がショートカットも持つ。メニューに無いショートカットには別の仕組みが要る。
3. `Action` を値として作り、`Actions` ウィジェットとメニューの両方に同じ値を渡す。

## 決定

3 を採る。Intent の段は Go では型スイッチか文字列になり、どちらも得が無い。

### 1. 型

```go
type Shortcut struct{ Key input.KeyCode; Modifiers input.Modifiers }  // String() は "Ctrl+Shift+S"
type Action struct {
	Label     string   // & でニーモニック。和文は "ファイル(&F)"
	Shortcut  Shortcut
	OnInvoke  func()   // nil なら無効。灰色になり、ショートカットも効かない
	Checkable, Checked bool
}
type Actions struct{ Actions []Action; Child Widget }
type MenuItem struct{ Action Action; Items []MenuItem; Separator bool }
type MenuBar struct{ Items []MenuItem }
type ContextMenu struct{ Items []MenuItem; Child Widget }
func ShowPopupMenu(ctx BuildContext, pos geom.Offset, items []MenuItem)
```

- `Action` は値なので、状態を持つ State の Build で作り、`Actions` とメニューの両方に同じ値を渡す。ID は持たない。`NSMenu` への写像（Phase 17）は位置で差分を取れば足りる。
- 無効は `OnInvoke == nil` で表す。`Button` の `OnPress` と同じ流儀で、否定形の bool（`Disabled`）を作らない。
- `MenuBar` は自分の項目のショートカットも受ける。メニューに出すキーのために `Actions` を別に書かなくてよい。メニューに無いキーだけ `Actions` に書く。

### 2. ショートカットの配送（core）

- `core.ShortcutTarget`（`HandleShortcut(*KeyEvent) bool`）を足した。`DispatchKey` は、フォーカスの FocusTarget の連鎖が使わなかったキーを、まずフォーカスの祖先の ShortcutTarget に近い順で、次にウィンドウ全体の残りに後に描いたものから渡す。
- 祖先だけにしなかったのは、何もフォーカスが無いときにも Ctrl+S が効いてほしいため。デスクトップでは、クリックで何も無い所を押すとフォーカスが外れる（ADR 0042）。
- モーダルの `FocusScope`（ダイアログ、開いたメニュー）があると、その中だけに配る。ダイアログの下で Ctrl+S が走らない。
- 押しっぱなしの繰り返しも配る（Ctrl+Z を押し続けると戻り続ける、Windows と同じ）。

### 3. Alt と F10

- `input.KeyAlt` を足した。**Alt を単独で押して離したとき**だけ、離した時点で `KeyAlt`（ModAlt なし）として配る。押しただけでは誰にも届かない。間に別のキー、ポインタの押下、ウィンドウのフォーカスの出入り（Alt+Tab）があれば取り消す。判定は `core.Tree` が持つ（`DispatchKeyUp`）。
- Windows は Alt と F10 で `WM_SYSKEYDOWN` / `WM_SYSKEYUP` を送り、`DefWindowProc` に渡すとシステムメニューのループに入る。gowaku は自前のメニューバーを描くので、`WM_SYSKEYUP` はすべて自分で受け、`WM_SYSCHAR` は捨てる（ビープを鳴らさない）。Alt+F4 と Alt+Space だけはシステムに残す。
- `input` に F1〜F12、メニューキー（`KeyContextMenu`）、全英字と数字、`, - . / =` を足した。X11 / Wayland は keysym の表、Windows は VK の表で写す。

### 4. メニューの動き（Windows に合わせる）

| 操作 | 動き |
|---|---|
| Alt 単独 / F10 | メニューバーの先頭にフォーカス。もう一度で戻る |
| Alt+ニーモニック | そのメニューを開き、先頭の項目にフォーカス |
| ←→ | バーの上で隣へ。開いたメニューでは隣のメニューへ（サブメニューの無い項目で → を押したとき） |
| ↑↓ Home End | 項目を移る。区切りは飛ばす。端で回り込む |
| → / Enter（サブメニュー） | サブメニューを開いて先頭へ |
| ← / Escape（サブメニュー） | 閉じて親の項目へ |
| Escape（最上段） | メニューを閉じてバーのタイトルへ。もう一度で元のフォーカスへ |
| 文字 | ニーモニックが合う項目を実行。無ければ頭文字の合う次の項目へ移る |
| ポインタ | タイトルの押下で開く。開いている間はタイトルの上に来るだけで隣のメニューへ。外を押すと閉じ、その押下は下に届かない |
| ウィンドウがフォーカスを失う | 閉じる。戻ってきたとき、元の場所にフォーカスがある |

- 項目は実際に `RenderFocus` を持ち、フォーカスが項目間を移る。こうしたのは、スクリーンリーダーに「どの項目にいるか」を読ませるため。ハイライトはフォーカスのある項目と、サブメニューが開いている親の項目。
- 項目を実行するときは、**先にメニューを閉じてフォーカスを戻し、それから `OnInvoke` を呼ぶ**。コピーが、フォーカスが戻った TextField を見つけられるようにするため。
- 閉じたあとも、次の Build まで項目は木に残る。その間にポインタが乗っても何もしない（セッションの `closed` で止める）。これを入れる前は、ポインタを離したときのホバーが、閉じたメニューの項目にフォーカスを戻していた。
- 右クリックは離したときに開く（Windows と同じ）。`GestureDetector.OnSecondaryTapAt` を足した。メニューキーと Shift+F10 は、フォーカスのある物の下に開く。テキスト欄のような背の高い物では、上端から 56px の所に開く。

### 5. 見た目（DESIGN.md の言語）

- 面は `Raised`、角丸 8、1px の `Hair`、影は `LiftFloat`、内側の余白 4。項目は高さ 28、角丸 4、フォーカスで `AccentTint`（無効の項目は `Fill`）。
- 左に幅 20 のチェックの列を全メニューが持つ。チェックの有無で文字の位置がずれないようにするため。ショートカットは右寄せで `Ink2`。サブメニューは `›`。
- 幅は、ラベルとショートカットの最も広いものを shaper で測って決める（180〜420）。layout には子に自然な幅を尋ねる仕組みが無い（Dialog と同じ事情）。
- バーは高さ 30、タイトルは高さ 26 の面で、開いている / キーボードで選んでいるときは `FillStrong`、ホバーは `Fill`。
- メニューはウィンドウの中に描く。ウィンドウの外にはみ出せない。下に入らなければ上に、右に入らなければ左に開き、それでも入らなければ端に寄せる。

### 6. アクセシビリティ

- ロールを足した：`RoleMenuBar`、`RoleMenu`、`RoleMenuItem`、`RoleMenuItemCheckbox`、`RoleSeparator`。UIA は MenuBar / Menu / MenuItem / Separator（チェック付きは MenuItem + Toggle）、AT-SPI は menu bar / menu / menu item / check menu item / separator。
- `semantics.Node.Shortcut` を足した。UIA は `AcceleratorKey`、AT-SPI は属性 `keyshortcuts`。バーのタイトルは "Alt+F" を出す。
- サブメニューのある項目とバーのタイトルは Expanded / Collapsed を持ち、ExpandCollapse で開閉できる。

## 結果

- widgets のテストで、Alt → 矢印 → Enter で実行してフォーカスが戻ること、Alt+F → R（ニーモニック）でサブメニュー、← / Escape の段の戻り方、無効の項目、ポインタでのメニュー間の移動、外の押下、ウィンドウのフォーカス喪失、Alt の間に別キーを挟んだときの取り消し、右クリックとメニューキーを確かめた。
- Windows の実ウィンドウで、`WM_SYSKEYDOWN`（F10）→ 矢印 → Enter の経路で、メニューの項目から ShowModal が開くことを確かめた。
- **残したもの:** ニーモニックの下線は描かない（text に下線が無い）。和文は括弧の英字で示す慣習なので困らないが、英語のラベルでは見えない。ポインタを押したまま項目に移って離すと選ぶ操作（press-drag-release）は未対応。サブメニューは乗った瞬間に開く（Windows の 400ms の遅延と、斜めに動かしたときの猶予は無い）。メニューの出入りのアニメーションは無い（Windows も既定では出るだけ）。同じフレーム内に 2 つのキーが届くと、2 つ目は開いたばかりのメニューの項目に届かない（自動操作でだけ起きる）。
