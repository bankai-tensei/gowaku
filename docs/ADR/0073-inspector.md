# 0073: Inspector

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画は Inspector を「`-tags gowakudebug` + F12 で別ウィンドウに Widget / Element / RenderObject ツリー、レイアウト境界と repaint の可視化、フレーム時間グラフ、DisplayList ダンプ、Semantics ツリー」とした。検討して形と範囲を決めた。

- 同じプロセスの別ウィンドウにする。gowaku 自身のウィジェットで描く。localhost のソケットで別プロセスから見る案と、アプリのウィンドウに重ねる案は採らなかった。
- 機能は「ピック + 閲覧」だけにする。値の書き換えはしない。テーマの切替やスロー再生もしない。
- フレームのグラフは、段階別（build / layout / paint / batch / present）の積み上げに、GPU 時間とアロケーション数を加える。遅いフレームをクリックすると、そのフレームで再ビルドされたものが見える。
- 開き方は 3 つ。`-tags gowakudebug` のビルドだけで動き、F12 で開閉する。`gowaku.OpenInspector()` でも開ける。タグ無しのビルドでは API は何もせず、Inspector のコードはリンクされない。

## 決定

### 1. タグ無しのビルドには何も残さない

- 定数で分ける。root パッケージは `inspecting`、core は `probing` という定数を、`gowakudebug` タグの有無で `true` / `false` にする（`inspector_on.go` / `inspector_off.go`、`core/probe_on.go` / `probe_off.go`）。フレームのフックは `if inspecting { ... }` の中に書くので、タグ無しではコンパイラが分岐ごと消す。
- `internal/inspect` は全ファイルに `//go:build gowakudebug` を付けた。タグ無しでは import もされない。
- `Tree.SetProbe` はタグ付きのビルドにだけ定義した。タグ無しでもあると、呼んでも何も起きない罠になる。
- 毎回残るのは `core.Tree` の `probe` フィールド（interface 1 つ、16 バイト）と、`app` の空の構造体だけ。
- 確認: タグ無しでビルドしたバイナリの `go tool nm` に `internal/inspect` と `SetProbe` が無いことをテストで確かめる。ホットパスの allocs/op が 0 のままなことも確かめる。

### 2. core に足したもの

- `core.Probe`（`Rebuilt` / `LaidOut` / `Recorded`）。呼ぶのは 3 箇所: element の rebuild と、render element の update（`element.go`）、`LayoutChild`（`renderobject.go`）、レイヤーの記録（`repaint.go` の `flush`）。どれも `probing && t.probe != nil` で守る。
- `core/inspect.go` は読み取り専用の API で、タグ無しでも使える。`ElementView`（比較できる値。TreeView の鍵にそのまま使える）、`Tree.RootElement`、`Tree.ElementOf(ro)`、`ConstraintsOf` / `NeedsLayoutOf` / `IsRelayoutBoundary` / `HasLayer`、`Tree.HostedBy`、`Tree.NeedsRepaint` がある。フレームの中からは呼ばない。gowakutest のような道具でも使える。
  - element を外に出さず、ビューを値で渡す。`element` の構造を公開 API に固定しないため。

### 3. 同じ UI ゴルーチンで、直接読む

- Inspector のウィンドウは、アプリの他のウィンドウと同じ `app` の 1 枚として開く。だから対象のツリーをロックもコピーもせずに直接読める。別プロセス案だと、ツリーを直列化する形式が要り、その形式が core の変化に追従し続ける負担になる。
- 対象は Inspector 自身を除く全ウィンドウ。複数あるときはツールバーのドロップダウンで選ぶ。ピックしたウィンドウには自動で切り替わる。
- Inspector の再ビルドは、新しいフレームの通知を 10 Hz に間引く（`notifyThrottled`）。Inspector 自身が描く時間で、測っている数字を乱さないため。選択とトグルの変化はすぐ反映する。

### 4. フレームの記録

- `Session` は 240 フレームの輪を持つ。1 フレームの記録は、段階ごとの時間、GPU 時間、アロケーション数、再ビルドした element（先頭 64 個と総数）、Layout した数、記録し直したレイヤー（先頭 64 個と総数）、全体を描き直したか、の 8 つ。
  - 輪のスロットはスライスを使い回す。記録中のアロケーションはアプリ自身の分だけにする。
- 段階の境目は `app.frame` の中で `time.Now` を取る。build と layout は `maxLayoutPasses` の繰り返しで交互に走るので、それぞれ足し込む。present には vsync の待ちが入る（凡例に書く）。
- アロケーション数は `runtime/metrics` の `/gc/heap/allocs:objects` の差で取る。`runtime.ReadMemStats` は世界を止めるので使わない。数はプロセス全体の分なので、他のゴルーチンの分も入る。
- GPU 時間: Inspector を開くときに、バックエンドが `EnableGPUTiming` / `GPUFrameTimes` を持っていれば（D3D11 と Vulkan。ベンチ用に既にある）有効にする。結果は数フレーム遅れて、全ウィンドウの present の順で返る。そこで全フレームに通し番号（`Seq`）を振り、待ち行列で対応させる。Inspector 自身のフレームは空の項目で場所だけ取る。輪がスロットを使い回した後に返ってきた時間は、Seq が合わないので捨てる。
- 「記録を止める」で輪を凍らせて、読む間に流れないようにできる。

### 5. オーバーレイ

- レイアウト境界、再描画の色付け、ピック中のホバー、選んだものの枠を、対象ウィンドウのキャンバスの最後に描く。
  - キャンバスは前のフレームの記録を使い回すので、オーバーレイがある間は毎フレーム `MarkNeedsPaint` して記録し直す。だから「全体を再描画した」の判定は、強制する前の `Tree.NeedsRepaint` で取る。
- 色はテーマに従わない固定色にした。明るいアプリと暗いアプリのどちらの上でも読めるようにするため。
- 再描画の可視化は、記録し直したレイヤーに薄い塗りと細い枠を付けて、500ms で消す。色は 1 色（落ち着いた紫）にした。消えるまでの間は `waitTime` に 16ms を返して、ループを起こす。
  - 最初は Flutter の repaint rainbow にならって、レイヤーごとに色相を回していた。全体を描き直したフレームでは、ウィンドウの縁も光らせていた。レビューで「虹色に光るのが嫌」と言われ、両方やめた。
  - 全体を描き直したかどうかは、フレームの詳細の「全体を再描画」で読む。縁の光は変化のたびにウィンドウ全体を光らせ、うるさい。
- 座標は `core.LocalRectToGlobal` で取る。回転の下では外接矩形になる。

### 6. ピック

- 「選択」をオンにすると、対象ウィンドウのポインタイベントを Inspector が先に受け取り、アプリには渡さない。ホバー中のものを `Tree.HitTest` の一番深いものとして示す。クリックで選んでピックを終え、Inspector のウィンドウを前に出す。Escape でやめられる。
- ピックの間は、アプリのボタンが押されたりフォーカスが動いたりしないことをテストで確かめる。

### 7. 見た目と言葉

- 構成は、上がツールバー（選択、レイアウト境界、再描画、記録を止める、ウィンドウ）、左が 3 つのツリーのタブ（Widget / RenderObject / Semantics）、右が詳細と DisplayList、下がフレームのグラフと詳細。
- Widget のツリーは element のツリーに widget の型名を付けて見せる。Flutter の Widget インスペクタと同じ見せ方。element の種類と状態は詳細に出す。
- 言葉は日本語と英語だけで、`widgets.Strings` には入れない。開発者のための道具で、アプリの一部ではないため。
- reflect: widget のフィールドを詳細に出すのに要るので、`internal/inspect` を depguard の reflect 禁止から外した。

### 8. 実機で決めたこと

- **内部を隠す**: gallery の「保存」のラベルは element の深さ 40 にある。TreeView の字下げではパネルの外に出る。
  - Widget ツリーは既定で、Inherited と、gowaku モジュールの非公開の型の element を隠す。隠した element の子は、その親の子として並べる。
  - 利用者の型は、非公開でも隠さない。モジュールの外にあるからである。
  - Flutter のように「利用者のコードが作ったものだけ」にするには、widget の生成位置の追跡が要る。Go では現実的でないので、この近似にした。
  - 「内部も表示」で全部出せる。
- **根を移す**: それでも深い選択（根から 9 段より下）では、ツリーの根を選択の 5 段上に移し、上にパンくずを出す。RenderObject ツリーも同じにした。
- **TreeView の修正**: レイアウトの前に `TreeController.Select` された行へのスクロールが捨てられていた。高さが分かるまで待つように直した。`widgets` 側の不具合なので、Inspector の外にも効く。
- **処理時間**: 要約、16.7ms を超えた数、グラフの目盛りは、present を除いた「処理」の時間で取る。present はほとんど vsync の待ちで、アイドルでも 16ms 近くになるため。棒には present も積み、目盛りを超える棒は上端で切って印を付ける。
- **間引いた通知を必ず届ける**: 10 Hz の間引きで、フレームが止まる直前の通知が落ちていた。落とした分は `Tick` で後から届ける。そのために `Wake` が待ち時間を返す。最初のフレームはすぐに届ける。
- **Windows の時計**: `time.Now` は約 0.5ms 刻みである。そこで段階の時間は `win32.PreciseTime`（QueryPerformanceCounter）で測る。`LazyProc.Call` は引数の指す先をヒープに出すので、カウンタの読み出しは `syscall.SyscallN` にし、周波数はキャッシュして、アロケーションを 0 にした。他の OS は `time.Since`。
- **GPU 時間は 1 フレームに 1 つ**: D3D11 は時計が乱れたフレームと、GPU が 8 フレーム遅れたときの最古の結果を捨てていた。Vulkan もクエリの失敗と輪（64）の溢れで捨てていた。1 つ欠けると、以降の時間がすべて 1 つずれて別のフレームに付く。そこで両方とも、測れなかったフレームには 0 を返すことにした（`GPUFrameTimes` の doc コメントに書いた）。0 は「不明」で、詳細では「—」と出す。ベンチ（`internal/shot`）は 0 を飛ばすので、結果は変わらない。

## 残したこと

- 値の書き換え、テーマの切替、スロー再生はしない（ヒアリングで決めた）。要るなら v3 以降で検討する。
- アロケーション数はプロセス全体の数なので、バックグラウンドのゴルーチンが多いアプリでは数字が膨らむ。
- GPU 時間は、デバイスのタイマーが輪（64）を溢れると対応がずれる。ポンプごとに回収しているので、実際には起きない。
- 別プロセスからの接続（アプリが固まっても見られる）は作らなかった。
