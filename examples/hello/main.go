// Command hello is the smallest gowaku application. It is also what the binary
// size in docs/BENCH.md measures.
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

type hello struct{ gowaku.StatelessBase }

func (hello) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.Text{Content: "こんにちは、gowaku"}}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "gowaku: hello", Width: 400, Height: 300, Home: hello{}})
	if err != nil {
		log.Fatal(err)
	}
}
