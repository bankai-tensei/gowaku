# 0002: gpu.Backend は NativeHandle だけを受け取り、Surface が Resize を持つ

決定日: 2026-11-07

状態: 採用

## 背景

当初の計画の `Backend.CreateSurface(w platform.Window)` は `platform.Window` を受け取る。一方 CODING_STYLE §3.1 は `gpu ─→ platform(NativeHandle のみ)` と定めている。両者が矛盾していた。また当初の計画にはリサイズの経路が無い。

## 選択肢

1. `platform.Window` を渡す。当初の計画どおりだが、`gpu` が Window の全メソッドに依存する。
2. `NativeHandle` とサイズを渡す。規約どおりで、テストでも窓の実体と切り離しやすい。

## 決定

選択肢 2。

```go
CreateSurface(h platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (Surface, error)

type Surface interface {
    Resize(physicalWidth, physicalHeight int, scale float32) error
    Destroy()
}
```

- `NativeHandle` は非公開メソッドを持つ閉じたインターフェースで、`platform.Win32Handle{HWND}` のような具体型を `platform` に置く。`gpu/d3d11` が `platform/win32` を import せずに済む。
- `Window.Size()` は `PhysicalSize()` にする。物理ピクセルを扱う名前には `physical` を含めるという規約 §4.1 に合わせた。
- `Resize` は空のサイズを無視する。最小化されたウィンドウは 0×0 を報告し、`ResizeBuffers` に 0 を渡すと別の意味になるため。
- `CreateTexture` / `UpdateTexture` はグリフアトラスが要る Phase 3 で入れる。`Batch` の形式は ADR 0005。

## 結果

当初の設計のコード片はこの ADR が上書きする。`platform.Platform` には ADR 0001 の `SetModalHandler` と `Destroy` が増えている。`IME`、`Clipboard`、`Cursor`、`PlatformBrightness` は必要になるフェーズで足す。
