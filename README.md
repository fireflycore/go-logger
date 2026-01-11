## go-logger

基于 zap 的轻量封装，提供：
- 控制台日志输出（面向人读）
- JSON 日志输出（写入自定义回调，面向机器解析）
- 可选异步写入器（队列满丢弃，避免阻塞业务线程）

## 安装

```bash
go get github.com/fireflycore/go-logger
```

## 快速开始

### 仅控制台输出

```go
package main

import logger "github.com/fireflycore/go-logger"

func main() {
	l := logger.New(&logger.Conf{
		Console: true,
		Remote:  false,
	}, nil)

	l.Info("hello")
}
```

### JSON 输出到回调

当 Remote=true 且提供了 handle 时，会把日志以 JSON bytes 的形式写入回调。

```go
package main

import logger "github.com/fireflycore/go-logger"

func main() {
	l := logger.New(&logger.Conf{
		Console: false,
		Remote:  true,
	}, func(b []byte) {
		_ = b
	})

	l.Info("hello")
}
```

### JSON 输出异步化

如果回调写入较慢，可以用 AsyncLogger 将写入异步化：

```go
package main

import logger "github.com/fireflycore/go-logger"

func main() {
	async := logger.NewAsyncLogger(1024, func(b []byte) {
		_ = b
	})

	l := logger.New(&logger.Conf{
		Console: false,
		Remote:  true,
	}, async.Logger)

	l.Info("hello")
}
```

## 配置说明

Conf 支持三个字段：
- Console：是否输出到 stdout（控制台 encoder）
- Remote：是否输出到回调（JSON encoder；需要同时提供 handle 才生效）
- Level：日志等级（debug/info/warn/error/panic；默认 info）

## 回调输出格式

当 Remote=true 且提供 handle 时，handle 会收到一段 JSON bytes，字段如下：
- Path：调用位置（file:line）
- Level：数字等级（兼容历史格式）
- Content：日志消息
- TraceId：可选字段（当日志 fields 中包含 trace_id 或 TraceId 时输出）
- CreatedAt：时间（time.DateTime 格式）

其中 TraceId 的常见写法：

```go
l.Info("hello", zap.String("trace_id", "xxx"))
```
