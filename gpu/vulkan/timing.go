//go:build linux || windows

package vulkan

import (
	"errors"
	"time"
	"unsafe"
)

// timerRing is how many frame times the timer keeps until GPUFrameTimes takes
// them.
const timerRing = 64

// timer measures the GPU time of frames with timestamp queries, two per frame in
// flight. It exists for benchmarks: a device has none until EnableGPUTiming.
type timer struct {
	pool VkQueryPool
	// written says that the commands of a frame in flight hold both stamps.
	written [framesInFlight]bool
	stamps  [2]uint64
	// times holds count results, the oldest at first.
	times        [timerRing]time.Duration
	first, count int
	// dropped counts the results that fell out of the ring before
	// GPUFrameTimes took them, which it reports as 0 ahead of the rest.
	dropped int
}

// EnableGPUTiming makes the device time each frame on the GPU, from the first
// command of its command buffer, which can be an upload or a DrawToTarget, to
// the last. GPUFrameTimes returns the results. It is meant for benchmarks. Call
// it outside a frame.
func (d *Device) EnableGPUTiming() error {
	if d.timer != nil {
		return nil
	}
	if !d.timestamps || d.timestampPeriod <= 0 {
		return errors.New("vulkan: the device cannot take timestamps")
	}
	if d.recording {
		return errors.New("vulkan: EnableGPUTiming called while commands are recorded")
	}
	info := VkQueryPoolCreateInfo{
		sType:      VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO,
		queryType:  VK_QUERY_TYPE_TIMESTAMP,
		queryCount: 2 * framesInFlight,
	}
	t := &timer{}
	r := callResult(d.vkCreateQueryPool, uintptr(d.device), ptr(&info), 0, ptr(&t.pool))
	if err := failure(r, "create query pool"); err != nil {
		return err
	}
	d.timer = t
	return nil
}

// GPUFrameTimes appends to times how long the GPU took for each frame whose
// fence has passed since the last call, oldest first, and returns the result.
func (d *Device) GPUFrameTimes(times []time.Duration) []time.Duration {
	t := d.timer
	if t == nil {
		return times
	}
	for ; t.dropped > 0; t.dropped-- {
		times = append(times, 0)
	}
	for ; t.count > 0; t.count-- {
		times = append(times, t.times[t.first])
		t.first = (t.first + 1) % timerRing
	}
	return times
}

// startTiming collects the stamps that f took the last time, whose fence has
// passed, and takes the first of this time. It records outside a render pass.
func (d *Device) startTiming(f *frame) {
	t := d.timer
	index := d.current
	if t.written[index] {
		t.written[index] = false
		r := d.hotResult(d.vkGetQueryPoolResults, uintptr(d.device), uintptr(t.pool), uintptr(2*index), 2,
			unsafe.Sizeof(t.stamps), ptr(&t.stamps), 8, VK_QUERY_RESULT_64_BIT|VK_QUERY_RESULT_WAIT_BIT)
		// A failed query still takes its place, as 0, so that the results
		// keep matching their frames by order.
		var d0 time.Duration
		if r == VK_SUCCESS && t.stamps[1] >= t.stamps[0] {
			d0 = time.Duration(float64(t.stamps[1]-t.stamps[0]) * float64(d.timestampPeriod))
		}
		if t.count == timerRing {
			t.first = (t.first + 1) % timerRing
			t.count--
			t.dropped++
		}
		t.times[(t.first+t.count)%timerRing] = d0
		t.count++
	}
	commands := uintptr(f.commands)
	d.hot(d.vkCmdResetQueryPool, commands, uintptr(t.pool), uintptr(2*index), 2)
	d.hot(d.vkCmdWriteTimestamp, commands, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, uintptr(t.pool), uintptr(2*index))
}

// stopTiming takes the last stamp of the frame.
func (d *Device) stopTiming(f *frame) {
	t := d.timer
	index := d.current
	d.hot(d.vkCmdWriteTimestamp, uintptr(f.commands), VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, uintptr(t.pool), uintptr(2*index+1))
	t.written[index] = true
}
