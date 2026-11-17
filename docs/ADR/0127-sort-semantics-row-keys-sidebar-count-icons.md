# 0127: 列見出しのソート状態、キーで行を追う選択、Sidebar の件数、標準のアイコン

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 4 つが見つかった。

1. **列見出しのソート状態：** `collection.Table` の列見出しは、並べ替えの向きを `Hint`（「昇順」など）で持っていた（ADR 0058）。Hint は説明の文字列であり、UIA の HelpText、AT-SPI の description として読まれるだけで、スクリーンリーダーには「この列で並んでいる」という状態として伝わらない。テストからも、文字列を比べるしかなかった。ARIA では `aria-sort` にあたる。
2. **行の番号による選択：** `TableController` は選択を `Table.Rows` の添字で持つ。並べ替えや行の削除の後も添字はそのまま残るので、強調表示が別のファイルに移る。godoc には「選択を消すか設定し直す」と書いてあったが、ファイル一覧のように並べ替えと削除が頻繁な画面では、利用者が毎回選択を付け直す必要があった。
3. **Sidebar の件数：** `collection.SidebarItem` には `Label`、`Icon`、`Selected`、`OnPress` しかなく、メールの未読数のような件数を置く場所がない。`control.Badge` の godoc は「サイドバーのフォルダの件数」に Quiet を勧めているのに、Sidebar の行には入れられなかった。
4. **標準のアイコン：** `nav.BreadcrumbItem.Icon` や `Tree.IconOf` は `*paint.Path` を求めるが、公開 API にアイコンが 1 つも無い。カタログのフォルダやファイルのアイコンは `internal/catalog/helpers.go` の非公開の関数で、利用者は SVG のパスデータを探してくるしかなかった。

## 選択肢

### ソート状態の持ち方

- **A. `semantics.State` に `SortAscending` と `SortDescending` のビットを足す。** State は uint16 で、14 ビットが使用済み。2 つ足すと埋まり、次の状態を足す余地が無くなる。「どちらも立っている」という意味の無い組み合わせも表現できてしまう。
- **B. `semantics.Node` に `Sort` フィールド（`SortNone`、`SortAscending`、`SortDescending`）を足す。** aria-sort と同じく 1 つの値であり、矛盾する組み合わせが無い。差分は `ChangedState` に含める（ソートの向きは状態であり、各プラットフォームは状態の変化として通知する）。
- **C. Hint のまま、文字列の書式を決める。** 読み上げはプラットフォームの言葉にならず、テストも文字列比較のまま。

### 各プラットフォームへの渡し方

- UIA にはソート専用のプロパティが無い。Chromium は aria-sort を `ItemStatus`（Narrator が読む）と `AriaProperties` の `sort=ascending` で渡している。
- AT-SPI は、Chromium が Orca に渡すのと同じオブジェクト属性 `sort`（`ascending` / `descending`）。
- NSAccessibility は `AXSortDirection`（`accessibilitySortDirection`、WebKit が aria-sort を VoiceOver に渡す方法）。

### 選択が行を追う方法

- **A. 選択をキーの集合として持つ。** `IsSelected(row)` は毎回キーを作って引くので、描画のたびに行の数だけ利用者の関数を呼ぶ。全選択は 100 万個のキーになる。
- **B. 添字の範囲を正とし、各 build でキーの写しと照らし合わせる。** 選択が変わった時点（利用者の操作の直後）と build の終わりに、選択中の行・フォーカス・Shift の起点のキーを記録する。次の build で、記録した位置に同じキーがまだあれば何もしない（ほとんどの build はこれで、選択中の行の数だけ呼ぶ）。違えば全行を一度走査して新しい位置を探す。行をその場で並べ替えても（同じスライスのまま）、記録は並べ替えの前に取ってあるので追える。
- **C. 行が変わったかを slice のヘッダで判定する。** その場の並べ替えではヘッダが変わらず、見逃す。

コードから `Select` などで選択を変えた場合は、その添字が「次の build の行」を指すものとして扱う（記録を捨て、build の終わりに取り直す）。行を消してから次の行を選ぶ、という書き方がそのまま動く。

### キーの API

- `Table[T]` は `RowKey func(row T) core.Key`。列の関数と同じく行の値から作る。`Key` という名前は Widget の `Key()` メソッドとぶつかるので使えない。
- `GridView` は行の型を持たないので `ItemKey func(i int) core.Key`。
- `List` は選択を持たないので対象外。`Tree` は最初からキーで選択を持つ。

### Sidebar の件数

- **A. `Trailing core.Widget`。** 何でも置けるが、Badge の文字が子のノードとして読まれ、行の名前と別に 2 度読まれる。読み上げの形は利用者任せになり、見た目もばらつく。
- **B. `Count int`。** 何も指定しなくても設計どおりの見た目になる（DESIGN.md §8）。Quiet の Badge と同じ灰色の丸い枠を行の右端に描き、99 を超えると「99+」。スクリーンリーダーには行の Value として完全な数を 1 度だけ伝え、枠そのものは読ませない。

### アイコンの置き場所

- **A. `widgets/basic` に入れる。** パスを作るだけのものが、ウィジェットの層に依存することになる。
- **B. 新しいパッケージ `icons` を paint のすぐ上に置く（paint と geom だけを import）。** ウィジェットの層からも、アプリからも使える。
- **C. `paint` 自身に入れる。** 描画の記録という paint の責任から外れる。

## 決定

- `semantics.Node.Sort`（型 `semantics.Sort`）を足す。`Table` の並べ替え中の列の見出しはこれを設定し、Hint の「昇順」「降順」はやめる。読み上げの言葉はプラットフォームに任せ、2 度読まれないようにする。使われなくなった `Strings.SortedAscending` と `SortedDescending` は削除する（未公開のため互換の層は作らない）。
- UIA は列見出しの `ItemStatus`（`ascending` / `descending`）と `AriaProperties`（`sort=...`）を返し、向きが変わったら ItemStatus のプロパティ変更を通知する。AT-SPI は属性 `sort`、NSAccessibility は列見出しにだけ `accessibilitySortDirection` を出す。
- 選択は案 B。`Table.RowKey` と `GridView.ItemKey` を足し、`TableController` に記録と照合（`remember` / `follow`）を持たせる。消えた行は選択から外れ、フォーカスの行が消えたときは同じ添字（その位置に来た行）に残る。全選択は「全部」として扱い、行が減っても増えても全部のまま。キーは比較でき、一意でなければならない。キーを与えない場合は従来どおり添字であり、行が変わったときに選択を消すのはアプリの仕事であることを godoc に書く。
- `SidebarItem.Count int` を足す（案 B）。
- 新しいパッケージ `icons` に、24 単位の格子の上で描いた 24 種のアイコンを置く：Folder、File、Document、Image、Search、Plus、Minus、Close、Check、ChevronLeft/Right/Up/Down、Settings、Trash、Edit、Copy、Home、Star、Info、Warning、Error、Menu、More。線は既存のウィジェットのアイコン（タブの閉じるボタン、ドックのピン）と同じ太さ 1.75、端と角は丸め、`Path.Stroke` で輪郭にして返す。Info などの点だけは塗りの円を足す。パッケージ変数は禁止なので関数で返し、呼ぶたびに新しいパスを作る（godoc に「一度作って持つ」と書く）。依存は `icons ─→ paint, geom` で、depguard に規則を足す。

## 結果

- 列見出しのソート状態がアクセシビリティのツリーで値として読め、テストも `Sort` を比べるだけで済む。NVDA が UIA の ItemStatus を読むかは確かめていない（Narrator と Chromium の慣例に合わせた）。
- `RowKey` を与えた表とグリッドは、並べ替え・絞り込み・削除の後も同じ行を選んだままになる。代償として、build のたびに選択中の行の数だけ `RowKey` を呼び、行が動いた build では全行の数だけ呼ぶ。`core.Key` への変換で割り当てが起きうるので、godoc ではポインターか小さい整数を返すよう勧めている。範囲を Shift で広げるたびに、選択中の行の数だけキーを取り直す。
- Sidebar に件数を出すのに `if` も Badge の組み立ても要らなくなった。
- カタログの `basic.Icon` の絵に 24 種の一覧を載せ、Sidebar、Tree、Breadcrumb、Button などの例を `icons` に置き換えた。対応するアイコンが無い四角・丸・三角だけが `internal/catalog/helpers.go` に残る。gallery の例も家・星・プラスを置き換え、雲だけは SVG のパスデータのまま残した。
