package controllers

import (
	"log"
	"net"
	"sync/atomic"
	"time"
)

// LatencyController measures real network latency by TCP-dialing a public host.
// It probes every 5 seconds and notifies a callback with each new measurement.
type LatencyController struct {
	stop      chan struct{}
	currentMs int64 // atomic; -1 = unreachable
}

func NewLatencyController() *LatencyController {
	return &LatencyController{
		stop:      make(chan struct{}),
		currentMs: 18, // shown before the first real measurement completes
	}
}

// Current returns the last measured latency in milliseconds, or -1 if unreachable.
func (lc *LatencyController) Current() int {
	return int(atomic.LoadInt64(&lc.currentMs))
}

// Start launches the background measurement loop.
// onUpdate is called from the goroutine each time a new value is ready;
// callers that need to update the UI must wrap it in QueueUpdateDraw.
func (lc *LatencyController) Start(onUpdate func(ms int)) {
	go func() {
		// Probe immediately so the first real value appears fast.
		lc.probe(onUpdate)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-lc.stop:
				return
			case <-ticker.C:
				lc.probe(onUpdate)
			}
		}
	}()
}

func (lc *LatencyController) probe(onUpdate func(ms int)) {
	ms := lc.measure()
	if ms >= 0 {
		atomic.StoreInt64(&lc.currentMs, int64(ms))
		if onUpdate != nil {
			onUpdate(ms)
		}
	}
}

// measure does a single TCP dial to 1.1.1.1:53 (Cloudflare DNS — always up,
// low overhead, no special permissions needed) and returns the round-trip time.
// Returns -1 on any error.
func (lc *LatencyController) measure() int {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 3*time.Second)
	if err != nil {
		log.Printf("LatencyController: probe failed: %v", err)
		return -1
	}
	conn.Close()
	return int(time.Since(start).Milliseconds())
}

// Stop shuts down the measurement goroutine cleanly.
func (lc *LatencyController) Stop() {
	select {
	case <-lc.stop: // already closed — do nothing
	default:
		close(lc.stop)
	}
}
