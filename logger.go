package logger

import (
	"encoding/json"

	"github.com/fireflycore/go-logger/internal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Conf 是 logger 的配置项。
// - Console：是否启用控制台输出
// - Remote：是否启用远端输出（需要同时提供 handle 才会生效）
type Conf struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`

	handle func(b []byte)
}

// WithHandle 设置远端输出回调。
//
// handle 会接收到经过本库二次整理的 JSON 字节串（outputLog 格式）。
func (c *Conf) WithHandle(handle func(b []byte)) {
	c.handle = handle
}

// Write 实现 io.Writer，用于把 zap 的 JSON 输出重定向到 handle。
//
// 这里不返回错误：日志写入失败不应影响业务逻辑（保持可用性优先）。
func (c *Conf) Write(b []byte) (int, error) {
	// 未开启远端回调时直接忽略写入（依然返回成功长度，避免上游误判）。
	if c == nil || c.handle == nil {
		return len(b), nil
	}

	// 解析 zap JSON 输出，提取必要字段并做字段名/等级转换。
	var data zapLog
	if err := json.Unmarshal(b, &data); err != nil {
		// 如果解析失败，直接透传原始字节，尽可能不丢日志。
		c.handle(b)
		return len(b), nil
	}

	// 组装为兼容下游的输出结构。
	out, err := json.Marshal(&outputLog{
		Path:      data.Path,
		Level:     levelToInt(data.Level),
		Content:   data.Message,
		TraceId:   data.TraceId,
		CreatedAt: data.CreatedAt,
	})
	if err != nil {
		// 序列化失败同样透传原始字节，避免完全丢失。
		c.handle(b)
		return len(b), nil
	}

	// 将结构化日志交给调用方处理（例如写入远端）。
	c.handle(out)

	return len(b), nil
}

// New 构造一个 zap.Logger。
//
// - Console=true 时输出到 stdout（面向人读）
// - Remote=true 且提供 handle 时输出 JSON 到 handle（面向机器解析）
// - 两者都未启用时返回 Nop logger，避免 nil 引用
func New(conf *Conf, handle func(b []byte)) *zap.Logger {
	// 允许传 nil：返回 nop，保持调用方简洁。
	if conf == nil {
		return zap.NewNop()
	}
	// handle 通过参数注入，便于一次性构造。
	if handle != nil {
		conf.handle = handle
	}

	// 多个 core 通过 Tee 合并，保证同一条日志可同时输出到多个目的地。
	cores := make([]zapcore.Core, 0, 2)
	if conf.Console {
		cores = append(cores, internal.NewConsoleCore())
	}
	// Remote 需要 conf.handle，否则无法写入，避免产生“启用但无输出”的隐式失败。
	if conf.Remote && conf.handle != nil {
		cores = append(cores, internal.NewJsonCore(conf))
	}

	// 没有任何输出目的地时返回 nop，避免 NewTee 空参数造成不可预期行为。
	if len(cores) == 0 {
		return zap.NewNop()
	}

	// AddCaller 会在日志中加入 caller 信息，字段名由 internal encoder 的 CallerKey 控制。
	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
