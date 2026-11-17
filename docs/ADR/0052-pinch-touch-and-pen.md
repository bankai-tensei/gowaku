# 0052: ピンチ、タッチ、ペン（14b）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「タッチ・ペン」と、Phase 14 の完了条件「タッチパッドのピンチで ListView がズームする」。ヒアリングで次を決めた。

- Windows のピンチは DirectManipulation で受け、使えない環境では Ctrl+ホイールとして扱う。
- この PC にはタッチ画面もペンも無い（高精度タッチパッドだけ）。タッチとペンは注入で確かめる。

## 決定

### 1. イベント

- `core.PointerPinch` と `PointerEvent.Pinch` を足した。Pinch は前のイベントからの倍率の変化で、指が離れると 1 を超える。`GestureDetector.OnPinch(factor, focus)` で受ける。
- **Ctrl+ホイールもピンチにする。** ピンチを受ける物があれば、倍率 1.1 の notch 乗のピンチとして渡す（ブラウザのズームと同じ）。受ける物が無ければ、今までどおりスクロールにする。Windows は DirectManipulation が無ければタッチパッドのピンチを Ctrl+ホイールにして送るので、これがその代わりにもなる。
- 装置の種類（`input.DeviceMouse / Touch / Pen`）、ペンの筆圧（0〜1）、指の ID をポインタイベントに載せた。

### 2. タッチ画面の扱い（core）

- 最初の指を今までのポインタとして配る。タップや押下はマウスと同じに動く。
- 指が 8px 以上動き、押されている物が譲るなら（`PanYielder`）、押下を取り消して、指の動きを押した場所へのスクロールに変える。ボタン（`RenderGesture`）は譲り、スライダーやドラッグは譲らない。指 100px がホイールの 1 notch に当たる。
- 2 本目の指が来たら押下を取り消し、2 本の距離の変化を中点でのピンチにする。
- `GestureDetector.OnLongPress(pos, device)` を足した。0.5 秒、8px 以内で押し続けると呼ばれ、その後に離してもタップにしない。`ContextMenu` は、指とペンの長押しでも開く（右ボタンが無いので）。
- 生のポインタイベントを受ける `Listener` を足した。押下、移動、離す、装置、筆圧をそのまま渡すので、お絵描きのような用途に使える。

### 3. プラットフォーム

| | ピンチ | タッチ | ペン |
|---|---|---|---|
| Windows | DirectManipulation | `WM_POINTER*`（PT_TOUCH） | `WM_POINTER*`（PT_PEN）、`GetPointerPenInfo` の筆圧 |
| X11 | XInput 2.4 の `XI_GesturePinch*` | XInput 2.2 の `XI_Touch*` | 未対応 |
| Wayland | `zwp_pointer_gestures_v1` | `wl_touch` | tablet-unstable-v2 |

- **DirectManipulation:**
  - ウィンドウごとにビューポートを作り、設定は INTERACTION | SCALING だけにする。平行移動まで頼むと、2 本指のスクロールが `WM_MOUSEWHEEL` で来なくなるため。
  - `DM_POINTERHITTEST` でタッチパッドの接触だけを渡す。
  - 動いている間は 16ms のタイマーで `Update` を呼ぶ。`OnContentUpdated` で、内容の変換行列から倍率を読む。
  - ピンチが終わると、内容を単位行列に戻す。
  - `ZoomToRect` は float を値で取るので、SyscallN では呼べない。代わりに `SyncContentTransform` を使う。
- **Windows のタッチとペン:** 自分で受けた `WM_POINTER` は返すので、マウスのメッセージに読み替えられない。マウスとタッチパッドの分は、今までどおりマウスのメッセージで来る。
- **X11:** libXi は任意で読み込む。サーバーが 2.2 未満なら、タッチもピンチも無い。
- **Wayland:**
  - プロトコルの XML を 2 つ足して、バインディングを作り直した（pointer-gestures-unstable-v1、tablet-unstable-v2）。
  - ペンのイベントは frame ごとにまとめて出す。

## 結果

- widgets のテストで、指のタップ、パンでのスクロール（押した行はタップされない）、2 本指のピンチ、長押し（1 回だけ、タップを食う）、Listener のペンの筆圧を確かめた。
- Windows：
  - DirectManipulation のビューポートが作れた。内容の倍率を 1.5、3 にすると、ピンチが 1.5、2 で届いた。終わると 1 に戻る。
  - `InjectSyntheticPointerInput` で注入したペンが、筆圧 0.5 → 0.75 つきで届いた。
  - `examples/zoom` のお絵描き欄に、注入したペンで描くと、筆圧に応じて太くなった（review の画像）。
- Wayland（sway）：`zwp_pointer_gestures_v1` を束ねて、ピンチのオブジェクトができた。コンポジタが送るはずのイベントを与えると、ピンチ、タッチ、ペンのイベントになった。
- X11（WSLg の XWayland）：XInput 2.3 なので、タッチはあり、ピンチは無い。構造体の配置は確かめた。
- **残したもの:**
  - 実際のタッチパッドのピンチは、指で試す必要がある（レビューで `examples/zoom` を試す）。
  - X11 のペンの筆圧（XInput の valuator の読み取り）。
  - 指のパンの慣性（離した後も滑る動き）。
  - 3 本目以降の指。
  - ペンの消しゴムとサイドボタン。
