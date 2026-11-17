package d3d11

import (
	"fmt"
	"time"
	"unsafe"
)

// timerSlots is how many frames the timer can wait on at once.
const timerSlots = 8

// timer measures the GPU time of frames with timestamp queries. It exists for
// benchmarks: a device has none until EnableGPUTiming.
type timer struct {
	slots [timerSlots]timerSlot
	// next is the slot of the frame in progress, and oldest that of the
	// oldest frame whose times are not in yet.
	next, oldest, waiting int
	// started means that the frame in progress has its first timestamp.
	started bool
	// dropped counts the frames given up on while the GPU was far behind,
	// which GPUFrameTimes reports as 0 ahead of the rest.
	dropped  int
	disjoint D3D11_QUERY_DATA_TIMESTAMP_DISJOINT
	stamp    uint64
}

type timerSlot struct {
	disjoint, begin, end *ID3D11Query
}

// EnableGPUTiming makes the device time each frame on the GPU, from its first
// command, which can be a DrawToTarget ahead of BeginFrame, to the end of
// EndFrame. GPUFrameTimes returns the results. It is meant for benchmarks, and
// costs a few queries per frame.
func (d *Device) EnableGPUTiming() error {
	if d.timer != nil {
		return nil
	}
	t := &timer{}
	for i := range t.slots {
		s := &t.slots[i]
		var err error
		if s.disjoint, err = d.device.createQuery(&D3D11_QUERY_DESC{Query: D3D11_QUERY_TIMESTAMP_DISJOINT}); err != nil {
			t.release()
			return fmt.Errorf("d3d11: %w", err)
		}
		for _, q := range []**ID3D11Query{&s.begin, &s.end} {
			if *q, err = d.device.createQuery(&D3D11_QUERY_DESC{Query: D3D11_QUERY_TIMESTAMP}); err != nil {
				t.release()
				return fmt.Errorf("d3d11: %w", err)
			}
		}
	}
	d.timer = t
	return nil
}

// GPUFrameTimes appends to times how long the GPU took for each frame that it
// has finished since the last call, oldest first, and returns the result. Each
// timed frame gets exactly one entry, so that the entries can be matched to
// frames by their order: a frame whose clock was disturbed, as by a change of
// power state, or whose result was given up on, gets 0.
func (d *Device) GPUFrameTimes(times []time.Duration) []time.Duration {
	t := d.timer
	if t == nil {
		return times
	}
	for ; t.dropped > 0; t.dropped-- {
		times = append(times, 0)
	}
	for t.waiting > 0 {
		duration, ok, ready := d.readSlot(&t.slots[t.oldest])
		if !ready {
			break
		}
		if !ok {
			duration = 0
		}
		times = append(times, duration)
		t.oldest = (t.oldest + 1) % timerSlots
		t.waiting--
	}
	return times
}

// readSlot reads the times of s, if the GPU has got that far.
func (d *Device) readSlot(s *timerSlot) (duration time.Duration, ok, ready bool) {
	t := d.timer
	// The pointers lead to fields of t, which outlive the calls.
	if d.context.getData(s.disjoint, unsafe.Pointer(&t.disjoint), uint32(unsafe.Sizeof(t.disjoint)), D3D11_ASYNC_GETDATA_DONOTFLUSH) != 0 {
		return 0, false, false
	}
	var begin, end uint64
	if d.context.getData(s.begin, unsafe.Pointer(&t.stamp), 8, D3D11_ASYNC_GETDATA_DONOTFLUSH) != 0 {
		return 0, false, false
	}
	begin = t.stamp
	if d.context.getData(s.end, unsafe.Pointer(&t.stamp), 8, D3D11_ASYNC_GETDATA_DONOTFLUSH) != 0 {
		return 0, false, false
	}
	end = t.stamp
	if t.disjoint.Disjoint != 0 || t.disjoint.Frequency == 0 || end < begin {
		return 0, false, true
	}
	return time.Duration(float64(end-begin) * 1e9 / float64(t.disjoint.Frequency)), true, true
}

// startTiming takes the first timestamp of a frame, once.
func (d *Device) startTiming() {
	t := d.timer
	if t == nil || t.started {
		return
	}
	if t.waiting == timerSlots {
		// The GPU is far behind; the oldest result goes.
		t.oldest = (t.oldest + 1) % timerSlots
		t.waiting--
		t.dropped++
	}
	s := &t.slots[t.next]
	d.context.begin(s.disjoint)
	d.context.end(s.begin)
	t.started = true
}

// stopTiming takes the last timestamp of the frame.
func (d *Device) stopTiming() {
	t := d.timer
	if t == nil || !t.started {
		return
	}
	s := &t.slots[t.next]
	d.context.end(s.end)
	d.context.end(s.disjoint)
	t.next = (t.next + 1) % timerSlots
	t.waiting++
	t.started = false
}

func (t *timer) release() {
	for i := range t.slots {
		s := &t.slots[i]
		for _, q := range []*ID3D11Query{s.disjoint, s.begin, s.end} {
			if q != nil {
				unknownOf(q).Release()
			}
		}
	}
	*t = timer{}
}
