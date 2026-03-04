package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"mosquitto-plugin/internal/pluginutil"
)

const defaultDispatchStopWait = 3 * time.Second

// queueWorker 管理队列协程及其生命周期。
// 发布函数和停止超时钩子在构造时注入，避免每次启动都传入可变依赖。
type queueWorker struct {
	mu sync.RWMutex

	queueCh chan []byte
	stopCh  chan struct{}
	doneCh  chan struct{}

	publish       func([]byte)
	stopWait      time.Duration
	onStopTimeout func(wait time.Duration, pending int)
}

// newQueueWorker 创建生产路径的队列工作协程管理器。
// 发布逻辑显式绑定 publisher/timeout，避免构造器分层与隐式默认行为。
func newQueueWorker(
	pub *amqpPublisher,
	publishTimeout time.Duration,
	stopWait time.Duration,
	onStopTimeout func(wait time.Duration, pending int),
) *queueWorker {
	// publishTimeout 兜底，避免误配置导致 context 立即超时。
	if publishTimeout <= 0 {
		publishTimeout = time.Second
	}
	if stopWait <= 0 {
		stopWait = defaultDispatchStopWait
	}
	if onStopTimeout == nil {
		onStopTimeout = defaultDispatchStopTimeout
	}
	publish := func(body []byte) {
		if pub == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
		defer cancel()
		if err := pub.Publish(ctx, body); err != nil {
			if pluginutil.ShouldSample(&workerWarnCounter, debugSampleEvery) {
				log(mosqLogWarning, "queue-plugin worker publish failed", map[string]any{"error": err})
			}
		}
	}
	return &queueWorker{
		publish:       publish,
		stopWait:      stopWait,
		onStopTimeout: onStopTimeout,
	}
}

// Start 启动（或重启）队列工作协程。
func (d *queueWorker) Start(buffer int) {
	d.Stop()

	queueCh := make(chan []byte, buffer)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	d.mu.Lock()
	d.queueCh = queueCh
	d.stopCh = stopCh
	d.doneCh = doneCh
	publish := d.publish
	d.mu.Unlock()

	go d.run(queueCh, stopCh, doneCh, publish)
}

func (d *queueWorker) run(queueCh <-chan []byte, stopCh <-chan struct{}, doneCh chan<- struct{}, publish func([]byte)) {
	defer close(doneCh)
	for {
		// 优先响应 stop，避免在高负载下退出被阻塞。
		select {
		case <-stopCh:
			return
		default:
		}

		select {
		case body := <-queueCh:
			publish(body)
		case <-stopCh:
			return
		}
	}
}

// Stop 停止队列工作协程，并在超时后触发告警钩子。
func (d *queueWorker) Stop() {
	d.mu.Lock()
	queueCh := d.queueCh
	stopCh := d.stopCh
	doneCh := d.doneCh
	stopWait := d.stopWait
	onStopTimeout := d.onStopTimeout
	d.queueCh = nil
	d.stopCh = nil
	d.doneCh = nil
	d.mu.Unlock()

	if stopCh == nil {
		return
	}
	pending := len(queueCh)
	close(stopCh)
	timer := time.NewTimer(stopWait)
	defer timer.Stop()
	select {
	case <-doneCh:
	case <-timer.C:
		if onStopTimeout != nil {
			onStopTimeout(stopWait, pending)
		}
	}
}

// Enqueue 按失败策略将消息写入内部队列。
func (d *queueWorker) Enqueue(body []byte, mode failMode, wait time.Duration) error {
	d.mu.RLock()
	queueCh := d.queueCh
	stopCh := d.stopCh
	d.mu.RUnlock()

	if queueCh == nil || stopCh == nil {
		return errDispatcherStopped
	}

	select {
	case <-stopCh:
		return errDispatcherStopped
	default:
	}

	switch mode {
	case failModeDrop, failModeDisconnect:
		select {
		case queueCh <- body:
			return nil
		case <-stopCh:
			return errDispatcherStopped
		default:
			return errQueueFull
		}
	case failModeBlock:
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case queueCh <- body:
			return nil
		case <-stopCh:
			return errDispatcherStopped
		case <-timer.C:
			return errEnqueueTimeout
		}
	default:
		return errQueueFull
	}
}

func defaultDispatchStopTimeout(wait time.Duration, pending int) {
	log(mosqLogWarning, "queue-plugin: dispatcher stop timeout", map[string]any{
		"wait_ms": int(wait / time.Millisecond),
		"pending": pending,
	})
}

var (
	errDispatcherStopped = errors.New("queue-plugin: dispatcher stopped")
	errQueueFull         = errors.New("queue-plugin: queue full")
	errEnqueueTimeout    = errors.New("queue-plugin: enqueue timeout")

	// defaultQueueWorker 在插件初始化时按最新配置创建。
	defaultQueueWorker *queueWorker
)
