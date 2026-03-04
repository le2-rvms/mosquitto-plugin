package main

import (
	"errors"
	"testing"
	"time"
)

// newQueueWorkerForTest 只在测试中使用，避免把测试注入接口暴露到生产代码。
func newQueueWorkerForTest(
	publish func([]byte),
	stopWait time.Duration,
	onStopTimeout func(wait time.Duration, pending int),
) *queueWorker {
	if publish == nil {
		publish = func([]byte) {}
	}
	if stopWait <= 0 {
		stopWait = defaultDispatchStopWait
	}
	if onStopTimeout == nil {
		onStopTimeout = defaultDispatchStopTimeout
	}
	return &queueWorker{
		publish:       publish,
		stopWait:      stopWait,
		onStopTimeout: onStopTimeout,
	}
}

func withDispatcherTestSetup(t *testing.T, fn func(d *queueWorker, release chan struct{})) {
	t.Helper()

	oldCfg := cfg
	release := make(chan struct{})
	d := newQueueWorkerForTest(func([]byte) { <-release }, defaultDispatchStopWait, defaultDispatchStopTimeout)

	cfg.enqueueTimeout = 20 * time.Millisecond
	cfg.publishTimeout = 20 * time.Millisecond
	d.Start(1)

	t.Cleanup(func() {
		close(release)
		d.Stop()
		cfg = oldCfg
	})

	fn(d, release)
}

func fillQueueUntilFull(t *testing.T, d *queueWorker) {
	t.Helper()
	for i := 0; i < 256; i++ {
		err := d.Enqueue([]byte("x"), cfg.failMode, cfg.enqueueTimeout)
		if errors.Is(err, errQueueFull) || errors.Is(err, errEnqueueTimeout) {
			return
		}
	}
	t.Fatal("queue did not become full in expected iterations")
}

func TestEnqueueMessageDropWhenFull(t *testing.T) {
	withDispatcherTestSetup(t, func(d *queueWorker, release chan struct{}) {
		cfg.failMode = failModeDrop
		fillQueueUntilFull(t, d)

		err := d.Enqueue([]byte("x"), cfg.failMode, cfg.enqueueTimeout)
		if !errors.Is(err, errQueueFull) {
			t.Fatalf("enqueue(drop) got err=%v want=%v", err, errQueueFull)
		}
	})
}

func TestEnqueueMessageDisconnectWhenFull(t *testing.T) {
	withDispatcherTestSetup(t, func(d *queueWorker, release chan struct{}) {
		cfg.failMode = failModeDisconnect
		fillQueueUntilFull(t, d)

		err := d.Enqueue([]byte("x"), cfg.failMode, cfg.enqueueTimeout)
		if !errors.Is(err, errQueueFull) {
			t.Fatalf("enqueue(disconnect) got err=%v want=%v", err, errQueueFull)
		}
	})
}

func TestEnqueueMessageBlockTimeout(t *testing.T) {
	withDispatcherTestSetup(t, func(d *queueWorker, release chan struct{}) {
		cfg.failMode = failModeBlock
		cfg.enqueueTimeout = 10 * time.Millisecond
		fillQueueUntilFull(t, d)

		start := time.Now()
		err := d.Enqueue([]byte("x"), cfg.failMode, cfg.enqueueTimeout)
		if !errors.Is(err, errEnqueueTimeout) {
			t.Fatalf("enqueue(block) got err=%v want=%v", err, errEnqueueTimeout)
		}
		if elapsed := time.Since(start); elapsed < 8*time.Millisecond {
			t.Fatalf("enqueue(block) timeout too short: %v", elapsed)
		}
	})
}

func TestEnqueueMessageStopped(t *testing.T) {
	d := newQueueWorkerForTest(func([]byte) {}, defaultDispatchStopWait, defaultDispatchStopTimeout)
	err := d.Enqueue([]byte("x"), failModeDrop, 10*time.Millisecond)
	if !errors.Is(err, errDispatcherStopped) {
		t.Fatalf("enqueue(stopped) got err=%v want=%v", err, errDispatcherStopped)
	}
}

func TestStopDispatcherBoundedWhenPublisherBlocked(t *testing.T) {
	oldCfg := cfg
	release := make(chan struct{})
	started := make(chan struct{})
	timeoutLogged := false
	d := newQueueWorkerForTest(
		func([]byte) {
			select {
			case <-started:
			default:
				close(started)
			}
			<-release
		},
		15*time.Millisecond,
		func(time.Duration, int) {
			timeoutLogged = true
		},
	)

	cfg.enqueueTimeout = 10 * time.Millisecond

	t.Cleanup(func() {
		close(release)
		d.Stop()
		cfg = oldCfg
	})

	d.Start(1)
	if err := d.Enqueue([]byte("x"), cfg.failMode, cfg.enqueueTimeout); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dispatcher did not start publish in time")
	}

	start := time.Now()
	d.Stop()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("stop took too long: %v", elapsed)
	}
	if !timeoutLogged {
		t.Fatal("expected timeout warning hook to be called")
	}
}
