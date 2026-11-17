//go:build windows || darwin

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
)

// libNames are the columns of the tables, in order.
var libNames = []struct{ key, title string }{
	{"gowaku", "gowaku"}, {"gio", "Gio"}, {"cogent", "Cogent Core"},
}

// metric is one row of a table: the median over the runs of one scenario.
type metric struct {
	title    string
	scenario string
	format   string
	value    func(*Result) float64
	// os is the only system whose files have the row, or empty for all.
	os string
}

var metrics = []metric{
	{"起動（ms）", "startup", "%.0f", func(r *Result) float64 { return r.StartupMs }, ""},
	{"アイドル CPU（%）", "idle", "%.2f", func(r *Result) float64 { return r.CPUPercent }, ""},
	{"アイドル GPU（%）", "idle", "%.2f", func(r *Result) float64 { return r.GPUPercent }, "windows"},
	{"アイドルのフレーム数（60 秒）", "idle", "%.0f", func(r *Result) float64 { return float64(r.Frames) }, ""},
	{"アイドルのパッケージ電力（W）", "idle", "%.1f", func(r *Result) float64 { return r.Watts }, "windows"},
	{"スクロール fps", "scroll", "%.1f", func(r *Result) float64 { return r.FPS }, ""},
	{"スクロール CPU（%）", "scroll", "%.1f", func(r *Result) float64 { return r.CPUPercent }, ""},
	{"スクロール GPU（%）", "scroll", "%.1f", func(r *Result) float64 { return r.GPUPercent }, "windows"},
	{"スクロール フレーム間隔 p99（ms）", "scroll", "%.1f", func(r *Result) float64 { return r.P99ms }, ""},
	{"アニメーション fps", "animate", "%.1f", func(r *Result) float64 { return r.FPS }, ""},
	{"アニメーション CPU（%）", "animate", "%.1f", func(r *Result) float64 { return r.CPUPercent }, ""},
	{"アニメーション GPU（%）", "animate", "%.1f", func(r *Result) float64 { return r.GPUPercent }, "windows"},
	{"アニメーション フレーム間隔 p99（ms）", "animate", "%.1f", func(r *Result) float64 { return r.P99ms }, ""},
	{"リサイズ 受けたサイズ / 秒", "resize", "%.0f", func(r *Result) float64 { return float64(r.Resizes) / r.Seconds }, "windows"},
	{"リサイズ 描いたフレーム / 秒", "resize", "%.0f", func(r *Result) float64 { return r.FPS }, "windows"},
	{"リサイズ CPU（%）", "resize", "%.1f", func(r *Result) float64 { return r.CPUPercent }, "windows"},
	{"リサイズ GPU（%）", "resize", "%.1f", func(r *Result) float64 { return r.GPUPercent }, "windows"},
	{"ピークメモリ: スクロール（MB）", "scroll", "%.0f", func(r *Result) float64 { return r.PeakMB }, ""},
	{"ピークメモリ: リサイズ（MB）", "resize", "%.0f", func(r *Result) float64 { return r.PeakMB }, "windows"},
	{"アイドル GPU 時間（%）", "idle", "%.2f", func(r *Result) float64 { return r.GPUPercent }, "darwin"},
	{"スクロール GPU 時間（%）", "scroll", "%.1f", func(r *Result) float64 { return r.GPUPercent }, "darwin"},
	{"アニメーション GPU 時間（%）", "animate", "%.1f", func(r *Result) float64 { return r.GPUPercent }, "darwin"},
	{"アイドル プロセスの CPU 電力（mW）", "idle", "%.1f", func(r *Result) float64 { return 1000 * r.ProcessWatts }, "darwin"},
	{"スクロール プロセスの CPU 電力（mW）", "scroll", "%.0f", func(r *Result) float64 { return 1000 * r.ProcessWatts }, "darwin"},
	{"アニメーション プロセスの CPU 電力（mW）", "animate", "%.0f", func(r *Result) float64 { return 1000 * r.ProcessWatts }, "darwin"},
	{"ピークメモリ: アイドル（MB）", "idle", "%.0f", func(r *Result) float64 { return r.PeakMB }, "darwin"},
	{"ピークフットプリント: アイドル（MB）", "idle", "%.0f", func(r *Result) float64 { return r.PeakFootprintMB }, "darwin"},
	{"ピークフットプリント: スクロール（MB）", "scroll", "%.0f", func(r *Result) float64 { return r.PeakFootprintMB }, "darwin"},
	{"ピークフットプリント: アニメーション（MB）", "animate", "%.0f", func(r *Result) float64 { return r.PeakFootprintMB }, "darwin"},
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	slices.Sort(values)
	return values[len(values)/2]
}

// report prints a results file as a Markdown table of medians.
func report(path string, w io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var results []Result
	if err := json.Unmarshal(data, &results); err != nil {
		return err
	}
	system := "windows"
	for i := range results {
		if results[i].OS != "" {
			system = results[i].OS
		}
	}
	fmt.Fprint(w, "| 項目 |")
	for _, l := range libNames {
		fmt.Fprintf(w, " %s |", l.title)
	}
	fmt.Fprint(w, "\n|---|")
	for range libNames {
		fmt.Fprint(w, "---:|")
	}
	fmt.Fprintln(w)
	for _, m := range metrics {
		if m.os != "" && m.os != system {
			continue
		}
		fmt.Fprintf(w, "| %s |", m.title)
		for _, l := range libNames {
			var values []float64
			for i := range results {
				r := &results[i]
				if r.Lib == l.key && r.Scenario == m.scenario && r.Failed == "" {
					values = append(values, m.value(r))
				}
			}
			fmt.Fprintf(w, " "+m.format+" |", median(values))
		}
		fmt.Fprintln(w)
	}
	return nil
}
