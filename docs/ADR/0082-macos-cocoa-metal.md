# 0082: macOS の土台（platform/cocoa と gpu/metal）

決定日: 2026-11-10

状態: 採用

## 背景

Phase 17（macOS と v2 公開）の最初の区切り。当初の計画の方針（purego/objc、`CAMetalLayer`、MSL の実行時コンパイル）で、Windows・Linux と同じアプリが macOS で動くところまでを作った。計画の見切り条件「2 週で三角形」は越えた。既存のサンプル（gallery、glass、rotate、tategaki、menus）が Metal で他の OS と同じ絵になる。

## 決定

### 1. gpu/metal は Vulkan の手順をそのまま写す

- 描画の手順（P1 の SDF クワッド、P2 のステンシル、P3 のカバー、クリップを深度の段で表す方式、ブレンドの背景コピー、3 パスのぼかし）は Vulkan と同じ。Metal は深度とステンシルの判定をパイプラインから分けて持つので、ステップごとの `MTLDepthStencilState` 6 個と、描画パイプライン（ブレンド状態ごとのクワッド、色を書かないクワッド、ファン）に分けた。
- 描画パスは 3 種類: サーフェス（MSAA 無し / 有り）と、`CreateTarget` のテクスチャ。テクスチャへの描画はパスもクリップも持たない（`DrawToTarget` の約束）ので、深度を持たない。
- MSL は HLSL（`gpu/shaders`）を手で 1 行ずつ移したもの（`gpu/metal/shaders/sdf.metal`）。spirv-cross は入れていないので生成はしない。HLSL を直したら MSL も直す。デバイス作成時に fast math を切ってコンパイルする。
- エンコーダーを途中で閉じるとき（背景コピー、ぼかし、`ReadPixels`）は、作成時のストアアクションを `MTLStoreActionUnknown` にしておき、閉じる直前に「続きがあれば保存、無ければ捨てる」を決める。MSAA の中身を毎フレーム保存しないで済む。
- テクスチャの更新はフレームのコマンドバッファの blit で行う（Vulkan と同じく、描画中のフレームと順序が保たれる）。Metal はコマンドバッファが参照中の資源を保持するので、Vulkan の「退役リスト」は要らない。
- autorelease pool は公開メソッドごとに張って閉じる。プラットフォームのイベントループにも pool があり、入れ子の順を崩さないため。pool はスレッドのものなので、その間だけ `LockOSThread` する（`New` は別ゴルーチンで走る。ADR 0018）。これを忘れると間欠的に落ちた。

### 2. ホットパスは整数引数の objc_msgSend、arm64 だけ

フレームごとの呼び出しは `purego.SyscallN(objc_msgSend, ...)` に整数とポインタだけを渡す（`objc.Send` は呼ぶたびに reflect で確保する）。16 バイトを超える構造体（`MTLViewport`、`MTLScissorRect`、`MTLOrigin`、`MTLSize`）は、AAPCS64 がそうするようにポインタで渡す。amd64 は値をスタックに積むので、この方法が使えない。そこで `gpu/metal` と `platform/cocoa` は **darwin/arm64 だけ**でビルドし、darwin/amd64 は他の未対応 OS と同じく `ErrUnsupported` を返す。Intel Mac は必要になったら、構造体の呼び出しを amd64 用に分けて足す。

浮動小数点レジスタに乗るもの（`MTLClearColor`、`setClearDepth:`）は `objc.Send` で、値が変わったときだけ呼ぶ。

### 3. platform/cocoa

- `NSView` のサブクラス（`NSTextInputClient`）とデリゲートを `objc.RegisterClass` で作る。メソッドは win32 のウィンドウプロシージャと同じく `Platform` のクロージャで、`Platform` ごとにクラス名を変える（通し番号はパッケージ変数 1 個）。
- `WaitEvent` は `nextEventMatchingMask:untilDate:` と `sendEvent:`。`Wake` は ApplicationDefined のイベントを `postEvent:` する（どのスレッドからでも安全）。Apple Event の終了要求のように、イベントを返さずにコールバックだけ起きるものがあるので、イベントの外から積んだときも同じ方法で待ちを起こす。
- 起動時に `[NSApp run]` を一度だけ回し、`applicationDidFinishLaunching:` で止める（GLFW と同じ）。こうしないと起動が完了した扱いにならない。
- AppKit はメインスレッドでしか動かないので、`init` で `runtime.LockOSThread` する。規約 §3.2 の「`init` に副作用を持たせない」の例外で、`main` より前でないと取れないため。
- ライブリサイズ中は AppKit がメインスレッドを持つので、`windowDidResize:` から modal handler を呼ぶ（ADR 0001 と同じ形）。
- 終了（⌘Q、Dock）は `applicationShouldTerminate:` で取り消し、全ウィンドウに `EventClose` を送る。アプリが閉じるかどうかを決める（ADR 0076）。
- キー: Command を `ModControl` に写す。Windows と Linux で Ctrl で書いたショートカット（コピー、貼り付け、元に戻す）がそのまま Mac の流儀で動く。Control キーも `ModControl`。Command のある打鍵は文字を打たない。`input.Modifiers` に Command を別に足すかは、メニューの表示（「⌘C」）を NSMenu に写すときに決め直す。
- IME: `setMarkedText:` の属性の太い下線を注目文節、細い下線を変換済みの文節、太い下線が無ければ全体を未変換とする（単体テスト有り）。IME が受け取ったキー（変換中の Enter など）はアプリに `EventKeyDown` として送らない。Windows の `VK_PROCESSKEY` と同じ振る舞い。
- スクロール: 精密なスクロール（タッチパッド、Magic Mouse）は 100pt を 1 ノッチにする。端数になるので ADR 0036 のとおり即時に動く。
- フォント: 既定のファミリーに「Hiragino Sans」を足した。

## まだ無いもの（Phase 17 の残り）

この節の項目は同日のうちに埋めた。カスタムシェーダー、NSMenu、カスタムタイトルバー、トレイと通知は ADR 0083、sbix 絵文字は ADR 0084、VoiceOver は ADR 0085、Finder との DnD は ADR 0050 の続きとして `platform/cocoa/drag_darwin.go`、arm64 で golden が合わなかった件は ADR 0086（FMA）。

## 影響

- `platform/cocoa` と `gpu/metal` を規約 §8 の「unsafe / purego を書いてよい場所」と `.golangci.yml` に足した。
- GPU テストはローカルで `go test -tags gpu ./gpu/metal/ ./platform/cocoa/`。Metal は窓の無い `CAMetalLayer` でも描けるので、`gpu/metal` のテストはウィンドウを作らない。`platform/cocoa` のテストは本物のウィンドウにイベントを合成して送る。
- macOS の `go test ./internal/golden` で `vertical` と `svg-icons` が合わなかったのは、arm64 の FMA が原因だった（ADR 0086 で修正）。
