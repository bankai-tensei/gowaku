# 0152: ほかの goroutine から State に仕事を届ける Poster

決定日: 2026-11-17

状態: 採用

## 背景

巨大なファイルを開く Editor（ADR 0153）では、行の索引づくりと検索を UI の goroutine の外で走らせ、結果を少しずつ画面に出したい。そのためには、ほかの goroutine から UI の goroutine に関数を届ける仕組みが要る。

あったのは `gowaku.Post` だけだった。これはルートのパッケージにあり、レイヤーの決まり（ADR 0081）で `widgets` からは import できない。また、`gowaku.Run` の外（`gowakutest` や `wtest`）では関数を捨てるので、テストで使えない。届いたときに State がもう木から外れているかどうかも、呼ぶ側で調べる必要があった。`StateBase.After` は UI の goroutine から呼ぶもので、ほかの goroutine からは使えない。

## 選択肢

- **A. `core.Tree` に投げ込みの列を持たせ、`StateBase.Poster()` で投げる関数を返す。** 列はミューテックスで守り、次の `Tick` の頭で走らせる。投げたときは、木を動かすループを起こす関数（`SetWake`）を呼ぶ。アプリは `platform.Wake` を、テストは自分の通知を渡す。
- **B. `StateBase.Post(fn)` を直接ほかの goroutine から呼べるようにする。** 呼ぶたびに `StateBase` の中を読むので、State が外れるのと競合する。race 検出器が指摘する形になる。
- **C. 仕事の間だけ Ticker を回し、チャネルを毎フレーム見る。** 新しい仕組みは要らないが、仕事の間ずっとフレームを回すことになり、待機中の CPU 0 % という性質と合わない。

## 決定

案 A。

- `func (b *StateBase) Poster() func(fn func())` を UI の goroutine で呼び、返った関数をほかの goroutine に渡す。返った関数は、どの goroutine から何度呼んでもよい。
- 投げた関数は、次のフレームの頭で、投げた順に、Ticker より先に走る。そのとき State が木から外れていれば捨てる。走っている途中で投げられた関数は、次のフレームに回す。
- 投げた関数が待っている間は、`Tree.Due` と `Tree.Ticking` が真を返す。アプリのループはこれを見て `Tick` を呼び、テストの `Settle` や `RunUntilIdle` も同じように進む。
- `Tree.SetWake(func())` で、投げたときに呼ぶ関数を決める。`gowaku` の窓は `platform.Wake` を渡すので、眠っているイベントループが起きる。
- テストで、ほかの goroutine の仕事を実時間で待つために、`gowakutest.Tester.WaitUntil(done, timeout)` と、ライブラリの中のテスト用に `wtest.WaitUntil` を足した。投げられるたびに 1 フレーム進め、`done` が真になるまで待つ。偽の時計は進めない。

## 結果

- テスト: 投げた順に次の Tick で走る、8 個の goroutine から 800 回投げても全部走る（race 検出器あり）、外れた State の仕事は捨てる、走っている途中で投げた仕事は次の Tick に回る、`WaitUntil` がほかの goroutine の仕事を待てる。
- `gowaku.Post` の godoc から `Poster` を案内した。
