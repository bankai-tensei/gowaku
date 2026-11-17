# 0116: 記号キーと OpenURL

決定日: 2026-11-14

状態: 採用

## 背景

gowaku で書いたコードと Markdown の閲覧アプリから二つの要望が来た。

- **#14：** `input` の記号キーは `, - . / =` だけ（ADR 0046）。`Ctrl+\`（VS Code の「右に分割」）や `Ctrl+@`（端末）を割り当てられない。`\ [ ] ; ' `` ` `` @` が欲しい。
- **#16：** アプリが `rundll32 url.dll,FileProtocolHandler` / `open` / `xdg-open` を `os/exec` で呼んでリンクを開いている。ライブラリの関数にしてほしい。

調べると、記号キーの意味がシステムごとにばらばらだった。

| | 今まで | JIS で `;`（+ のキー）を押すと |
|---|---|---|
| Windows | 仮想キー（`VK_OEM_PLUS` → `KeyEqual`） | `KeyEqual` |
| X11 / Wayland | 修飾なしの keysym（今の配列の文字） | `KeyNone`（`;` を知らない） |
| macOS | ANSI 配列の物理位置（`kVK_ANSI_*`） | `kVK_ANSI_Quote` の位置なので `KeyNone` |

英字と数字はどこでも「そのキーの英字・数字」で、Windows の仮想キーも配列に従う（AZERTY の A のキーは `VK_A`）。

## 決定

### 1. 記号の KeyCode は「修飾なしで今の配列が打つ ASCII の文字」

- 物理位置ではなく文字にする。物理位置（US の名前）にすると、JIS の `@` キーは `KeyBracketLeft` になり、「Ctrl+@」というショートカットを書けない。メニューに出す文字と押すキーも食い違う。文字なら、アプリが `KeyAt` と書けば JIS では `@` キー、Linux の keysym も macOS のメニューの key equivalent もこの考えで動いている。
- **Shift が要る文字は、それを打つキーと `ModShift` で届く。** US で `@` は `Shift+Key2`。`KeyAt` のショートカットは US では押せない。端末のように US の Ctrl+@（Ctrl+Shift+2）も拾いたいアプリは、両方を受ける。Shift 付きの文字で KeyCode を決めると、`Ctrl+Shift+Z` が `Z` で届く今の約束（keysym を修飾なしで引く、ADR 0046）と食い違うのでしない。
- **ASCII の記号はすべてコードを持つ**（値は文字そのもの）。名前のある定数は `KeyQuote` `KeyComma` `KeyMinus` `KeyPeriod` `KeySlash` `KeySemicolon` `KeyEqual` `KeyBracketLeft` `KeyBackslash` `KeyBracketRight` `KeyBackquote`（ここまで US の記号キー）、`KeyAt` `KeyColon` `KeyCaret`（JIS で単独のキー）、`KeyPlus` `KeyAsterisk` `KeyHash` `KeyLess`（テンキーとドイツ・フランス配列）。ほかの記号（AZERTY の `!` `$` など）も文字のコードで届き、`String()` はその文字を返す。
- **`input.PunctuationKey(r rune) KeyCode`** を足した。ASCII の記号ならそのコード、ほかは `KeyNone`。プラットフォームが「キーの打つ文字」を渡す共通の入口。
- **¥ は `KeyBackslash`。** 日本語では同じ文字として扱われ、Windows の JIS 配列は ¥ キーで `\`（0x5C）を打つが、Mac の ¥ キーは U+00A5 を打つ。どちらでも `Ctrl+\` が効くように。JIS の ろ キーも `\` を打つので `KeyBackslash`。
- **ASCII の外の文字を打つキー**（ドイツの ü など）は `KeyNone`。英字と数字のキーは今までどおり（Windows は仮想キー、macOS は ANSI の位置、Linux は keysym）で、AZERTY の数字の段は Windows では `Key1`…、Linux では `&` などの記号になる（下の「残したこと」）。
- **テンキーの演算子** は配列に依らず `KeyAsterisk` `KeyPlus` `KeyMinus` `KeySlash`。小数点はロケールで `.` か `,` になるので割り当てない。
- **デッドキー**（US 国際配列の `'` `` ` `` `^`、ドイツの `^`）はアクセントの文字として扱う。Windows の `MapVirtualKey` はデッドキーに最上位ビットを立てて文字を返すので、ビットを落とす。Linux は `dead_grave` → `` ` ``、`dead_acute` → `'`、`dead_circumflex` → `^`、`dead_tilde` → `~`。

### 2. システムごと

- **Windows：** `VK_OEM_1`〜`VK_OEM_8` と `VK_OEM_102`、`VK_OEM_PLUS/COMMA/MINUS/PERIOD` を、`MapVirtualKeyW(vk, MAPVK_VK_TO_CHAR)`（スレッドのキーボード配列で、修飾なしの文字）で文字にし、`PunctuationKey` に渡す。`WM_KEYDOWN` / `WM_SYSKEYDOWN` ごとに 1 回の呼び出しで、アロケーションは無い。`keyCodeOf(vk, typed func(uintptr) rune)` にして、テストで US（kbdus）と JIS（kbd106）の表を差し込む。
  - US：`VK_OEM_5` → `\`、`VK_OEM_4` → `[`、`VK_OEM_3` → `` ` ``。JIS：`VK_OEM_3` → `@`、`VK_OEM_4` → `[`、`VK_OEM_6` → `]`、`VK_OEM_1` → `:`、`VK_OEM_PLUS` → `;`、`VK_OEM_7` → `^`、`VK_OEM_5`（¥）と `VK_OEM_102`（ろ）→ `\`。
  - テストはこの表のほか、この機械に入っている US / 日本語の配列を `MapVirtualKeyExW` で引いて表と照らし（この機械は kbd106 で一致）、`keyEvent` に `VK_OEM_5` の `WM_KEYDOWN` を通して `KeyBackslash` が届くのを確かめる。Ctrl の状態は `GetKeyState` から読むので単体テストでは押せないが、キーの決め方は Ctrl に依らない（`MapVirtualKey` は修飾を見ない）。
- **X11 / Wayland：** keysym はもともと修飾なしで今の配列から引いている（X11 は `XLookupKeysym(ev, 0)`、Wayland は xkbcommon の `keymap_key_get_syms_by_level` のレベル 0）。`platform/internal/keysym` が Latin-1 の keysym（値が文字のコード）を `PunctuationKey` に渡すようにした。jp 配列では `@` `[` `]` `:` `^` が単独の keysym、¥ と ろ は `backslash`。テンキーの演算子とデッドキーも足した。
- **macOS：** 記号のキー（ANSI の記号の位置と `kVK_JIS_Yen` `kVK_JIS_Underscore`）だけ、`charactersByApplyingModifiers:0`（macOS 10.15。Go が要る macOS 11 より古い）で修飾なしの文字を聞く。JIS のキーボードでは `@` のキーが ANSI の `[` の位置（`kVK_ANSI_LeftBracket`）にあり、位置では決められないため。文字が空なら（イベントが言わないなら）ANSI の位置の文字に戻る。英字と数字は今までどおり物理位置。テンキーの記号は表のまま。メニューの key equivalent は ASCII の記号をすべてそのまま出す（`⌘\`、`⌘@`）。
- **ショートカットの表示：** `KeyCode.String()` が記号を文字のまま返すので、メニューやコマンドパレットは `Ctrl+\`、`Ctrl+@`、`Ctrl+[` と出る。`widgets/basic` にはキーの名前を持つ場所が無かった。

### 3. `gowaku.OpenURL(u string) error`

- **置き場所：** ルート。ウィンドウも `Run` も要らず、どのゴルーチンから呼んでもよい。アプリが既に import しているパッケージで、`ShowFileDialog` と並ぶ「システムに頼む」関数。
- **範囲：** URL だけ。ファイルやフォルダーを既定のアプリで開くのは `file:` の URL で出来る（Windows の ShellExecute、macOS の NSWorkspace はそのまま、Linux はポータルか xdg-open）。「フォルダーで表示」（選んだ状態で Explorer / Finder を開く）は別の仕事なので、要望が来たら別の関数にする。
- **検査：** 絶対 URL だけ受け、ほかは `ErrInvalidURL`（`errors.Is` で引ける）。空、スキームが無い（`example.com`、`/docs`、`//host`）、1 文字のスキーム（`C:\docs` や `C:/docs` はドライブ文字がスキームに読まれる）、スキームの後が空（`https:`）、`//` があるのにホストが無い（`https:///x`。`file:` は除く）、空白と制御文字、`url.Parse` が失敗するもの（`%zz`、先頭の `-`）。先頭が文字なので `xdg-open` がオプションと取ることも無い。
- **スキームは制限しない。** 許可リストの引数も持たない。アプリごとに要るスキームが違い（その閲覧アプリは http / https / mailto、ほかは `vscode:` や独自のスキーム）、引数にすると全員が書くことになる。代わりに doc コメントに、`file:` のプログラムは Windows では走ること、文書の中のような信用できないリンクはアプリがスキームを絞るべきことを書き、`url.Parse` でスキームを見る例を置いた。
- **シェルを通さない。** Windows は `ShellExecuteW`（"open"）、macOS は `NSWorkspace openURL:`、Linux は D-Bus と `exec.Command("xdg-open", u)`（引数として渡す）。
- **Windows：** ShellExecute はスキームのハンドラーを COM で呼ぶことがあり、ドキュメントは COM の初期化を求める。呼び出したスレッドの COM の状態は分からないので、`LockOSThread` した自前のゴルーチンで `CoInitializeEx(APARTMENTTHREADED | DISABLE_OLE1DDE)` してから呼び、`S_OK` / `S_FALSE` なら `CoUninitialize` する。戻り値 32 以下は `windows.ShellExecute` がエラーにする。
- **macOS：** `NSURL URLWithString:` が nil ならエラー、`openURL:` が NO ならエラー。NSWorkspace はどのスレッドからでも使えるので、メインスレッドに回さず、自前の autorelease pool の中で呼ぶ。
- **Linux：** まず xdg-desktop-portal の `org.freedesktop.portal.OpenURI.OpenURI("", uri, {handle_token})`（`platform/dbus.OpenURI`。既存の session bus と `call` を使う）。ポータルが無い、または失敗したら `xdg-open`。両方だめなら両方のエラーを `errors.Join` で返す。ポータルの Response（アプリの選択を取り消したか）は待たない。選ぶ画面が出ている間、呼び出しが止まってしまうので。`xdg-open` も `Start` だけして待たない（デスクトップが無いと xdg-open がブラウザーを自分で走らせ、閉じるまで返らない）。終了は別のゴルーチンで刈り取る。
- **順に試す仕組み：** `openers []func(string) error` を OS ごとに返し（Windows と macOS は 1 つ、Linux はポータルと xdg-open、ほかの OS は空で `ErrUnsupported`）、テストは偽の opener で順番とエラーの合わせ方を確かめる。

## 結果

- 閲覧アプリは `Ctrl+\`（US でも JIS でも）と JIS の `Ctrl+@` を `interact.Shortcut{Key: input.KeyBackslash, ...}` のように書ける。メニューには `Ctrl+\` と出る。
- リンクは `gowaku.OpenURL(link)` になる。スキームの許可はアプリに残る。
- **振る舞いの変わるところ：** Windows の JIS 配列で `;`（+）のキーは `KeyEqual` だったのが `KeySemicolon` になる。`KeyEqual` で拡大する `interact` のビューアーと examples は、JIS の Windows で Ctrl+;（+ のキー）では拡大しなくなる（JIS の `=` は Shift+- なので `KeyMinus` で届き、`KeyEqual` を打つキーが無い）。Firefox のように拡大に `KeySemicolon` と `KeyPlus`（テンキーの + とドイツ配列）も受けるのが良い。マージのときに `interact` のビューアーをそう直した（Ctrl と `KeyEqual` / `KeySemicolon` / `KeyPlus` で拡大。US では Ctrl+; でも拡大する）。examples のメニューの Ctrl+= は 1 つのショートカットしか持てないので、JIS の Windows では届かないまま。macOS の JIS で `^` のキーが `KeyEqual` だったのも `KeyCaret` になる。

## 残したこと

- AZERTY の数字の段は、Linux では修飾なしの keysym が `&` などなので `Key1`… にならない。キリル文字などの配列で Linux の英字が `KeyNone` になるのも同じ。US の位置に戻す仕組み（別のグループの keysym を引く）は別にする。
- macOS の英字は今までどおり ANSI の物理位置で、AZERTY や Dvorak の英字は配列に従わない（Windows と Linux は従う）。メニューの key equivalent は AppKit が文字で照らすので、そちらは配列に従う。
- 実機での確認：Windows で `OpenURL("https://example.com")` を一度呼び、既定のブラウザー（Chrome）が開いた。macOS と Linux の OpenURL、macOS の `charactersByApplyingModifiers:` と Linux の jp 配列は、この機械では走らせていない（クロスコンパイルと vet、OS に依らない単体テストだけ）。
