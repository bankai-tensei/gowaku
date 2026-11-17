# 0126: ダイアログ内の入力欄のフォーカス、複数行の欄の Enter による送信、時間をおいた実行

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 3 つが見つかった。

1. **ダイアログ内の入力欄にフォーカスが移らない。** メニュー項目から「名前の変更」ダイアログ（`overlay.ShowDialog` の `Dialog{Content: field.Input{Controller: &ctl}}`）を開き、`ctl.Focus()` を `ShowDialog` の前に呼んでも後に呼んでも、入力欄はフォーカスを得なかった。どちらの場合も、入力欄はまだできていないので、`Focus` は「次にレイアウトされたときに受け取る」という予約になる。ダイアログは自分を `interact.Focus{Autofocus: true}` で包んでいる。`RenderFocus` は子をレイアウトした後で `RequestFocus` を呼ぶので、子の入力欄が予約どおりに受け取ったフォーカスを、直後にダイアログが取り上げていた。ライブラリのテスト（`widgets/field/dialog_focus_test.go`）は、`Pump` の後に `Focus` を呼んでいたので、この問題に気づかなかった。アプリは `Pump` を呼べない。あわせて、入力欄ができる前の `InputController.SelectAll` は、コントローラーがまだ文字列を知らないので、何も選択しなかった。
2. **複数行の欄で Enter と Shift+Enter を区別できない。** 複数行の `field.Input` は、修飾キーにかかわらず Enter で改行を挿入し、`OnSubmit` は 1 行の欄でしか呼ばれない。`Extras.onKey` は非公開である。そのため、「Enter で送信し、Shift+Enter で改行する」というチャットの入力欄が作れなかった。キーイベントの修飾キーは、全プラットフォームで入力欄まで届いている。
3. **一度だけの遅延実行が無い。** 「1.2 秒後に実行する」を、`core.Ticker`、`SleepUntil`、`Stop` を組み合わせて作っていた。`time.AfterFunc` と `gowaku.Post` で作ると、実時間で動くのでテストの時計（`gowakutest.Advance`）では進まず、`Post` は `Run` の外では関数を破棄する。

## 選択肢

### 1. ダイアログ内のフォーカス

- (a) 入力欄に `Autofocus bool` だけを足す。宣言的で分かりやすいが、`Focus()` を先に呼ぶ自然なコードと、`docs/guide/*/recipes.md` が勧める書き方は壊れたままになる。
- (b) `RenderFocus` の Autofocus を、子のレイアウト中にフォーカスが自分の内側へ移っていたら取らないように変える。予約された `Focus()` がダイアログに勝つ。シートや、同じ仕組みを使う他の Autofocus にも効く。
- (c) (a) と (b) の両方。

### 2. Enter による送信

- (a) `Extras.onKey` を公開して、アプリにキーを見させる。IME の確定や Form との連携を、アプリが毎回書くことになる。
- (b) `field.Input` に `SubmitOnEnter bool` を足す。複数行の欄で、Enter が `OnSubmit`（無ければ Form の送信）を呼び、Shift+Enter が改行する。
- (c) `OnSubmitKey` のように、送信するキーの組み合わせを選べるようにする。用途が少ないわりに API が増える。

### 3. 遅延実行

- (a) `core.StateBase.After(d, fn) (cancel func())`。State の Ticker として作るので、State の寿命に従って止まる。
- (b) `motion` に `Timer` 型を足し、`Attach` で State に付ける。`AnimationController` と同じ形だが、一度きりの呼び出しには手順が多い。
- (c) Tree に時計（現在時刻を返す関数）を持たせ、`After` が呼ばれた時刻から数える。`NewTree` はウィジェットをすぐに展開するので、時計を設定する前に `InitState` が走り、そこでの `After` は時計を使えない。

## 決定

1 は (c)、2 は (b)、3 は (a) を採る。

### 1. ダイアログ内のフォーカス

- **`RenderFocus`：** Autofocus は、子をレイアウトする前のフォーカスを覚えておき、子のレイアウト中にフォーカスが自分の内側へ移っていたら `RequestFocus` を呼ばない。内側の要求の方が具体的だからである。`OnAutofocus` には、子をレイアウトする前のフォーカスを渡す。ダイアログやシートが閉じたときに戻す先は、入力欄ではなくダイアログを開く前のもの（メニューを開いたボタンなど）になる。
- **`field.Input.Autofocus`：** 最初にレイアウトされたときにキーボードを受け取る。ダイアログ、シートの Autofocus より優先される。宣言的に書きたいときはこちらを使い、開く前に選択も決めたいとき（名前の変更）は `InputController` の `SelectAll` と `Focus` を使う。両方を残した。
- **`InputController.SelectAll`：** 入力欄ができる前に呼ぶと、その入力欄ができたときの全文を選択する。それまでは、コントローラーが覚えている最後の文字列の長さで選択していたので、新しいコントローラーでは何も選択しなかった。
- **シート：** 動きを減らす設定（ReduceMotion）では、シートが不透明度 0 から現れる。不透明度 0 の部分木はフォーカスを持てない（`core.Hider`）ので、最初のフレームで受け取ったフォーカスが次のフレームで外れていた。ダイアログと同じく、現れる途中は `enteringOpacity` より薄くしない。

### 2. Enter による送信

- **`field.Input.SubmitOnEnter`：** 複数行の欄で、Enter は `OnSubmit` を呼び、`OnSubmit` が無ければ外側の Form を送信する。Shift+Enter は改行する。Ctrl や Alt との組み合わせも、Shift が無ければ送信する。Shift の有無だけで分けるのが、チャットの入力欄で最も多い約束事である。
- **IME：** 変換中の Enter は、各プラットフォームがアプリに渡さない（Windows は `VK_PROCESSKEY`、macOS は `interpretKeyEvents`、X11 と Wayland は入力メソッドが取る）。それでも変換中の Enter が届いたときは、入力欄は何もせず、確定を IME に任せる。`SubmitOnEnter` の有無にかかわらず、変換中の Enter で送信も改行もしない。IME が確定した文字列に含まれる改行は、複数行の欄なのでそのまま残る。
- **1 行の欄の Shift+Enter：** これまでどおり送信する。ブラウザの `<input>` も、Windows と macOS の 1 行の欄も、Shift+Enter で既定の動作（送信）をするので、それに合わせた。godoc に明記した。

### 3. 遅延実行

- **`StateBase.After(d time.Duration, fn func()) (cancel func())`：** `d` が経った後に、UI ゴルーチンで `fn` を一度だけ呼ぶ。State の Ticker なので、State がツリーを離れると取り消され、`fn` が破棄済みの State で `SetState` を呼ぶことはない。`cancel` はそれより前に取り消す。`fn` が走った後や 2 回目の `cancel` は何もしない。
- **時刻の起点：** Ticker の最初の呼び出しの時刻から数え、そこから `SleepUntil` で眠る。コールバックの中で呼べば、同じ pump の Tick が最初の呼び出しになるので、呼んだ時刻とほぼ同じである。gowakutest でも、`Tap` の後の `Pump` が同じ時刻で Tick するので、押した時刻から数える。`InitState` の中で呼んだときは、そのフレームの Tick がすでに終わっているので、次の Tick から数える。gowakutest では、`New` の中の `InitState` は最初の Tick より前に走るので、時刻 0 から数える。
- **アイドル時のコスト：** 最初の呼び出しのために、もう 1 回だけイベントループを回る（フレームは描かない。`NeedsFrame` は偽のまま）。その後は眠っている Ticker なので、待っている間はフレームを描かない。
- **(c) を採らなかった理由：** 時計を持たせても、`NewTree` 中の `InitState` には効かず、二通りの起点ができる。今の方法で、呼んだ直後の Tick との差は 1 フレーム未満である。
- **`NewPeriodicTicker` の godoc：** 最初の呼び出しは 1 周期後ではなく、`Start` の次のフレームですぐに来ることを明記し、一度だけの遅延には `After` を案内した。
- **ドキュメント：** `gowaku.Post` と gowakutest のパッケージ doc に、遅延には `After` を使うことを書いた。

## 結果

- メニューやボタンのコールバックから開いたダイアログやシートで、`Focus` を `ShowDialog` の前に呼んでも後に呼んでも、入力欄がキーボードを受け取る。`SelectAll` を先に呼べば、全文を選択して開く。`TestDialog_FieldAskedForTheKeyboardBeforeItExistsGetsIt` で、前・後・Autofocus のそれぞれを、ダイアログとシートで確かめる。
- `TestInput_SubmitOnEnter*` で、Enter の送信、Shift+Enter の改行、変換中の Enter で送信しないこと、確定した改行が残ることを確かめる。`TestInput_EnterWithoutSubmitOnEnter` で、これまでの動作（複数行は改行、1 行は Shift+Enter でも送信）を固定した。
- `gowakutest/after_test.go` で、`After` が `Advance` で遅延のちょうど後に走ること、待つ間 Ticking でないこと、`InitState` からの呼び出し、取り消し、State がツリーを離れたときの取り消しを確かめる。
- ガイドの concepts（Ticker と UI ゴルーチン）、text-and-ime（入力欄の項目）、recipes、testing-and-preview と、カタログの `field.Input` を更新した。
