# 0063: Navigator、PageRoute、Hero（15c）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 §2 で、Navigator の範囲を次のとおり決めた。

- Push / Pop / Replace のページスタックと、`"/notes/42"` のような名前付きルート
- 戻るは Alt+← とマウスの戻るボタン
- 遷移は控えめなスライドとフェード

当初の計画には、ページの間で要素が飛ぶ `Hero` もある（`examples/notes` の、検索結果からの遷移）。

## 決定

### 1. API

- `Navigator{Home, Initial, Routes []RoutePattern, NotFound, Controller, OnChange}`。`NavigatorOf(ctx)` で、いちばん近い `NavigatorController` を得る（`OverlayOf` と同じく、無ければ panic）。
- コントローラの操作:
  - `Push`、`PushNamed`、`Replace`、`ReplaceNamed`
  - `Pop`、`PopUntil`、`CanPop`
  - `Forward`、`CanForward`
  - `Top`、`Depth`
- `PageRoute{Name, Page, Transition, Instant, OnPop}`、`RoutePattern{Pattern, Build, Transition, Instant}`、`Hero{Tag, Child}`、`BackButton{Label}`。

### 2. 名前付きルート

- **パターンの表は、最初に一致したものを使う。** 「いちばん具体的なもの」より、読んだ順のほうが結果を予想しやすいため。
  - `:name` は 1 つの区間、最後の `*` は残り全部に一致する。末尾の `/` は区別しない。
  - `?` の後ろの問い合わせは一致に使わない。`RouteParams.Get` は、パスの値を先に、無ければ問い合わせを読む。
- 名前で積んだページは、Navigator が作り直されるたびに、今の `Routes` から作り直す。アプリの状態から作るページが古くならないようにするため。値で積んだページ（`PageRoute.Page`）は変わらない。
- `Initial: "/notes/42"` は、そのページだけを積む。親のページまでは作らない。
- **知らない名前でも panic しない。** 「ページが見つかりません」のページ（または `NotFound`）を積む。名前は利用者の入力から来ることがあるため。
- **積んだページから値を返す仕組みは作らない。** Go には future が無いので、結局コールバックになる。値を返すページには、積む側が型付きの `OnSave func(T)` を渡せばよい。`OnPop` は、どの経路で去っても呼ばれる。

### 3. 戻ると進む

- `Forward` の積み直しを覚えるのは、利用者が戻ったとき（Alt+←、マウスの戻るボタン、`BackButton`）だけ。プログラムの `Pop` では覚えない。Push や Replace をすると忘れる。
- **Alt+← / Alt+→ は、フォーカスより先にナビゲーションの対象へ渡す**（core の `NavigationTarget`）。そうしないと、TextField、TextEditor、TreeView が矢印を自分で使ってしまう。
  - 渡す順は、フォーカスの祖先を近い順に、それから窓の残りを、後から描いたもの・内側を先にたどる。隠れた部分木は飛ばす。modal の範囲があれば、その中に閉じる。
  - どの対象も戻れなければ、キーはいつもどおりフォーカスへ行く。
- **マウスの親指ボタンは押下として配らない。** 離したときに、ポインタの下の対象へ深い順に渡す。だから Navigator の上のダイアログが止める。ボタンを確かめない部品が、クリックと取り違えることもない。
- 各 OS の対応:

  | OS | 戻る | 進む | 備考 |
  |---|---|---|---|
  | Win32 | `WM_XBUTTON*` の XBUTTON1 | XBUTTON2 | TRUE を返して `DefWindowProc` を呼ばない。`WM_APPCOMMAND` が重ならないようにするため |
  | X11 | ボタン 8 | ボタン 9 | |
  | Wayland | BTN_SIDE / BTN_BACK | BTN_EXTRA / BTN_FORWARD | |

- Escape では戻らない。Backspace も使わない（ブラウザもやめた）。

### 4. 遷移

- 進むときは 280ms の `motion.Standard`、戻るときは 200ms の `motion.Leave`。24px のスライドにフェードを重ね、下のページは動かさずに見せておく。
  - スプリングにしなかったのは、SpringOut が 17% ほどの時間で行き着いて約 10% 行き過ぎ、ページ全体が揺れて見えたため。
- 入ってくる途中のページを戻すと、同じ曲線で逆向きに戻る（跳ばない）。
- ReduceMotion ではフェードだけを残し、スライドと Hero の飛行はやめる。
- **標準の遷移は描き直すだけで、Build し直さない。**
  - ページは RepaintBoundary に入れ、新しい `SetOpacity` で 1 枚の絵としてフェードさせる。レイヤーはフレームごとに使い回し、フェードも図形ごとではなくグループの不透明度になる。
  - スライドは、ページの枠の中で描く位置をずらすだけ。
  - 1 フレームのアロケーションは 0（テストで確かめる）。Hero が飛んでいるフレームで約 420ns。
- 自前の `Transition` を渡すと、AnimatedSwitcher と同じく毎フレーム Build し直す。

### 5. 下のページとフォーカス

- 一番上でないページは、State を保ったまま `core.Hider` で隠す。ヒットテスト、Tab、Semantics から外れ、覆われた後は描かない。
- 去っていくページはすぐに入力を失い、現れるページはすぐに入力を受ける。
- ページは、自分の中でフォーカスを持っていたものを覚える。戻ったらそこへ返し、無ければ最初の部品へ。
- 入ってきたページはフォーカスを受ける（Autofocus があればそれ、無ければ最初の部品。たいてい BackButton）。**このフォーカスの移動で、スクリーンリーダーにページが変わったことが伝わる。**
- Navigator の入れ子では、内側が先に戻る。内側の最初のページにある BackButton は、外側で戻る。BackButton は戻る先があるページにだけ出る。

### 6. Hero

- 組は遷移の始まりに探す。入ってくるページの Hero と下のページの Hero を、タグで照らし合わせる（隠れた部分木は除く）。同じページに同じタグがあれば panic する。
- 位置は毎フレーム Navigator からの相対で測る。だから、スクロールしたリストの中の Hero も、見えている場所から飛び立つ。
- 矩形は遷移の曲線に沿って動く。Hero はページの上に、Navigator の範囲で切って描く。2 つの子を重ねてフェードし、縦横を同じ比で拡大するので、文字の形が崩れない。飛んでいる間は元の 2 つを隠す。
- 飛行は動いているページのものなので、Push を途中で戻すと、自然に飛び戻る。

### 7. BackButton の幅（統合で直したもの）

gallery に置いたところ、ラベルの無い BackButton が列の幅いっぱいに広がり、矢印がページの真ん中に出た。中の `Center` が列の幅を取っていたため。中身の幅にした（テストあり）。

## 結果

- golden は `navigator-push`、`navigator-hero`、`navigator-detail`（それぞれライトとダーク）。前の 2 つは遷移の途中（280ms のうち約 80ms）。新しい `Scene.Frames` で、何フレーム進めるかを指定する。
- `examples/navigator` を Windows の実機（D3D11）で、SendInput で動かした。
  - 行を押すと、Hero が飛んで詳細のページが積まれる。
  - 本物の XBUTTON1 で戻り、XBUTTON2 で進む。
  - Alt+← で戻ると、フォーカスが行に戻る。BackButton でも Hero が飛び戻る。
- gallery の「ナビゲーション」で、UIA と Orca（X11）で確かめた。
  - 積むとフォーカスが「メモ」のボタンへ移り、下の一覧は木から消える。
  - Alt+← で戻ると、押した行（「京都旅行」）にフォーカスが戻る。
  - Alt+→ で進むと、また「メモ」のボタンへ移る。
- 残り:
  - ルートごとの長さの指定。
  - 右から左の書字方向（MediaQuery に書字方向が無い）。
  - 指でのスワイプで戻る操作。
  - キーボードのブラウザの戻るキー（`VK_BROWSER_BACK` / `WM_APPCOMMAND`）。
  - 積むたびに下のページの Widget を 1 回作り直す（State は保つ。遷移のフレームでは作り直さない）。
  - 拡大の下や別のレイヤーの中のページは、図形ごとのフェードになる。
  - Linux の親指ボタンは単体テストだけ。
