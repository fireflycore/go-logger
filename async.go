package logger

// AsyncLogger 用于把日志写入异步队列，然后由后台 goroutine 调用 handle 消费。
//
// 该类型同时实现 io.Writer，可直接作为 zap 的写入目标。
type AsyncLogger struct {
	// queue 为日志缓冲队列；满时丢弃，避免阻塞业务线程。
	queue chan []byte
	// handle 为实际的写入回调（例如发送到远端、写文件等）。
	handle func(b []byte)
}

// NewAsyncLogger 创建一个异步写入器。
//
// size 为队列长度；当队列已满时，新日志会被丢弃（不阻塞调用方）。
func NewAsyncLogger(size int, handle func(b []byte)) *AsyncLogger {
	logger := &AsyncLogger{
		queue:  make(chan []byte, size),
		handle: handle,
	}

	// 后台消费者协程：单线程消费，保证 handle 调用串行。
	go logger.init()

	return logger
}

func (l *AsyncLogger) init() {
	for b := range l.queue {
		if l.handle != nil {
			l.handle(b)
		}
	}
}

// Write 实现 io.Writer。
//
// 这里会复制入参切片，避免上层复用/修改同一底层数组导致数据竞争或内容错乱。
func (l *AsyncLogger) Write(p []byte) (n int, err error) {
	select {
	case l.queue <- append([]byte(nil), p...):
	default:
	}
	return len(p), nil
}
