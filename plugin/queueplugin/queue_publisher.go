package main

import (
	"context"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpPublisher 管理连接/通道并负责重连。
type amqpPublisher struct {
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel

	dsn            string
	exchange       string
	routingKey     string
	publishTimeout time.Duration

	nextDial time.Time
}

// newAMQPPublisher 基于当前配置创建发布器实例，避免方法内部依赖全局 cfg。
func newAMQPPublisher(cfg config) *amqpPublisher {
	return &amqpPublisher{
		dsn:            cfg.dsn,
		exchange:       cfg.exchange,
		routingKey:     cfg.routingKey,
		publishTimeout: cfg.publishTimeout,
	}
}

// Reset 关闭并清理发布器状态；用于插件重载和退出场景。
func (p *amqpPublisher) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
	p.nextDial = time.Time{}
}

// Warmup 预热连接和通道，失败时会回收半初始化资源。
func (p *amqpPublisher) Warmup() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureLocked(); err != nil {
		p.closeLocked()
		return err
	}
	return nil
}

func (p *amqpPublisher) closeLocked() {
	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

func (p *amqpPublisher) ensureLocked() error {
	if p.conn != nil && p.conn.IsClosed() {
		p.conn = nil
		p.ch = nil
	}
	if p.ch != nil && p.ch.IsClosed() {
		p.ch = nil
	}
	if p.conn == nil {
		if !p.nextDial.IsZero() && time.Now().Before(p.nextDial) {
			return errors.New("queue-plugin: reconnect backoff")
		}
		conn, err := amqp.DialConfig(p.dsn, amqp.Config{
			Dial: amqp.DefaultDial(p.publishTimeout),
		})
		if err != nil {
			p.nextDial = time.Now().Add(1 * time.Second)
			return err
		}
		p.nextDial = time.Time{}
		p.conn = conn
		log(mosqLogInfo, "queue-plugin: connected to rabbitmq", nil)
	}
	if p.ch == nil {
		ch, err := p.conn.Channel()
		if err != nil {
			_ = p.conn.Close()
			p.conn = nil
			return err
		}
		p.ch = ch
		log(mosqLogDebug, "queue-plugin: channel opened", nil)
	}
	return nil
}

// Publish 发送消息，如果连接/通道关闭会重试一次。
func (p *amqpPublisher) Publish(ctx context.Context, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureLocked(); err != nil {
		return err
	}

	err := p.ch.PublishWithContext(ctx, p.exchange, p.routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err == nil {
		return nil
	}

	if errors.Is(err, amqp.ErrClosed) || (p.conn != nil && p.conn.IsClosed()) || (p.ch != nil && p.ch.IsClosed()) {
		p.closeLocked()
		if err2 := p.ensureLocked(); err2 != nil {
			return err
		}
		return p.ch.PublishWithContext(ctx, p.exchange, p.routingKey, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	}

	return err
}
