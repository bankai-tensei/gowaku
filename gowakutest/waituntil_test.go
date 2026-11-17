package gowakutest_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// counting counts to 50 on a goroutine of its own and posts each step.
type counting struct{ core.StatefulBase }

type countingState struct {
	core.StateBase
	n int
}

func (counting) CreateState() core.State { return &countingState{} }

func (s *countingState) InitState() {
	post := s.Poster()
	go func() {
		for i := 1; i <= 50; i++ {
			time.Sleep(time.Millisecond)
			post(func() { s.SetState(func() { s.n = i }) })
		}
	}()
}

func (s *countingState) Build(core.BuildContext) core.Widget {
	return basic.Text{Content: "count " + strconv.Itoa(s.n)}
}

func TestTester_WaitUntilRunsPostedWork(t *testing.T) {
	tt := gowakutest.New(t, counting{}, gowakutest.Options{})
	tt.WaitUntil(func() bool { _, ok := tt.Find("count 50"); return ok }, 0)
}
