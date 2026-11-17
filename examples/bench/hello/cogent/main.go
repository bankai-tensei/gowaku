// Command cogent is the hello of Cogent Core, for the binary size in docs/BENCH.md.
package main

import "cogentcore.org/core/core"

func main() {
	b := core.NewBody("hello")
	core.NewText(b).SetText("こんにちは")
	b.RunMainWindow()
}
