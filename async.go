package logger

import (
	"sync"
)

// AsyncLogger 用于把日志写入异步队列，然后由后台 goroutine 调用 handle 消费。
//
// 该类型同时实现 io.Writer，可直接作为 zap 的写入目标。
type AsyncLogger struct {
	// queue 为日志缓冲队列；满时丢弃，避免阻塞业务线程。
	queue chan []byte
	// handle 为实际的写入回调（例如发送到远端、写文件等）。
	handle func(b []byte)
	// closed 关闭信号：Close 后 Write 会直接丢弃，后台协程会 drain 队列后退出。
	closed chan struct{}
	// once 确保 Close 只执行一次，避免重复 close channel panic。
	once sync.Once
}

// NewAsyncLogger 创建一个异步写入器。
//
// size 为队列长度；当队列已满时，新日志会被丢弃（不阻塞调用方）。
func NewAsyncLogger(size int, handle func(b []byte)) *AsyncLogger {
	// 兜底队列长度，避免无缓冲导致业务线程阻塞。
	if size <= 0 {
		size = 1
	}
	logger := &AsyncLogger{
		// queue 仅用于缓存日志 bytes；满时丢弃以保护业务线程。
		queue:  make(chan []byte, size),
		handle: handle,
		// closed 用于通知后台协程退出，并在退出前尽量处理完队列中已有日志。
		closed: make(chan struct{}),
	}

	// 后台消费者协程：单线程消费，保证 handle 调用串行。
	go logger.init()

	return logger
}

func (l *AsyncLogger) init() {
	// 采用 select 同时监听队列与关闭信号：
	// - 常态从 queue 消费
	// - 收到关闭信号后 drain 队列并退出，避免 goroutine 泄漏
	for {
		select {
		case b := <-l.queue:
			// handle 为 nil 时直接丢弃，保持行为可控。
			if l.handle != nil {
				l.handle(b)
			}
		case <-l.closed:
			// 收到关闭信号：继续把队列里已有日志尽量消费完，再退出。
			for {
				select {
				case b := <-l.queue:
					if l.handle != nil {
						l.handle(b)
					}
				default:
					return
				}
			}
		}
	}
}

// Write 实现 io.Writer。
//
// 这里会复制入参切片，避免上层复用/修改同一底层数组导致数据竞争或内容错乱。
func (l *AsyncLogger) Write(p []byte) (n int, err error) {
	// nil 接收者直接当作写入成功，减少调用方判空分支。
	if l == nil {
		return len(p), nil
	}
	// 已关闭时直接丢弃（返回成功长度，避免影响业务逻辑）。
	select {
	case <-l.closed:
		return len(p), nil
	default:
	}
	select {
	// 入队前复制 slice，避免上层复用底层数组导致内容错乱或数据竞争。
	case l.queue <- append([]byte(nil), p...):
	default:
		// 队列满：丢弃，保证不会阻塞调用方。
	}
	return len(p), nil
}

func (l *AsyncLogger) Close() {
	// Close 为可选调用：允许 nil 接收者。
	if l == nil {
		return
	}
	// 只关闭一次，避免重复关闭 channel 导致 panic。
	l.once.Do(func() {
		close(l.closed)
	})
}

func (l *AsyncLogger) Sync() error {
	// Sync 的语义为“尽量落盘/落库”：本实现通过 Close 触发 drain 并退出。
	l.Close()
	return nil
}
