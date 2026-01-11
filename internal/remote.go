package internal

import (
	"encoding/json"
	"time"

	"go.uber.org/zap/zapcore"
)

type remoteLog struct {
	// Path 是触发日志的调用位置（trim 后的 file:line）。
	Path string `json:"Path"`
	// Level 是数字等级，保持与历史下游解析兼容。
	Level int `json:"Level"`
	// Content 是日志消息内容（对应 zap 的 entry.Message）。
	Content string `json:"Content"`
	// TraceId 可选字段：从 fields 中提取 trace_id/TraceId。
	TraceId string `json:"TraceId"`
	// CreatedAt 是日志时间，使用可读的 time.DateTime 格式。
	CreatedAt string `json:"CreatedAt"`
}

type remoteCore struct {
	// level 控制该 core 允许输出的最小日志等级。
	level zapcore.LevelEnabler
	// handle 是远端写入回调：接收 JSON bytes。
	handle func(b []byte)
	// fields 为通过 Logger.With(...) 挂载的“常驻字段”。
	fields []zapcore.Field
}

// NewRemoteCore 构造一个远端输出 core。
//
// 该 core 的目标是减少额外编解码：直接在 core.Write 中组装目标 JSON，并调用 handle。
func NewRemoteCore(level zapcore.LevelEnabler, handle func(b []byte)) zapcore.Core {
	return &remoteCore{
		level:  level,
		handle: handle,
		fields: nil,
	}
}

func (c *remoteCore) Enabled(level zapcore.Level) bool {
	// zap 会先调用 Enabled 判断是否需要写入。
	return c.level.Enabled(level)
}

func (c *remoteCore) With(fields []zapcore.Field) zapcore.Core {
	// With 用于在 Logger.With(...) 时挂载字段，返回一个新的 core（保持无共享写入）。
	if len(fields) == 0 {
		return c
	}
	// 值拷贝保留旧 core 的配置，再复制并追加字段，避免修改原切片带来的数据竞争。
	next := *c
	next.fields = append(append([]zapcore.Field(nil), c.fields...), fields...)
	return &next
}

func (c *remoteCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// Check 是 zap 的快速路径：只有 Enabled 的日志才会进入 Write。
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *remoteCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// handle 未设置时直接忽略写入，避免影响业务。
	if c.handle == nil {
		return nil
	}

	// allFields 合并 With(...) 挂载的字段与本次日志携带字段，便于统一提取 trace_id。
	allFields := fields
	if len(c.fields) != 0 {
		allFields = append(append([]zapcore.Field(nil), c.fields...), fields...)
	}

	// traceId 从 fields 中提取，优先匹配标准 snake_case（trace_id），兼容历史的 TraceId。
	traceId := ""
	for _, f := range allFields {
		if (f.Key == "trace_id" || f.Key == "TraceId") && f.Type == zapcore.StringType {
			traceId = f.String
			break
		}
	}

	// path 优先使用 zap 提供的 Caller（需要上层 zap.AddCaller()）。
	path := ""
	if entry.Caller.Defined {
		path = entry.Caller.TrimmedPath()
	}

	// 将 entry 映射到下游期待的字段结构，避免先 JSON 编码再反序列化的额外开销。
	b, err := json.Marshal(&remoteLog{
		Path:      path,
		Level:     levelToInt(entry.Level),
		Content:   entry.Message,
		TraceId:   traceId,
		CreatedAt: entry.Time.Format(time.DateTime),
	})
	// JSON 序列化失败时丢弃该条日志（不返回错误，保持日志不影响业务）。
	if err == nil {
		c.handle(b)
	}
	return nil
}

func (c *remoteCore) Sync() error {
	// handle 是否需要 flush 由 handle 自己保证；此处保持无副作用。
	return nil
}

func levelToInt(level zapcore.Level) int {
	// 该映射保持与旧版本一致：下游存储/检索可能依赖数字等级。
	switch level {
	case zapcore.InfoLevel:
		return 1
	case zapcore.WarnLevel:
		return 3
	case zapcore.ErrorLevel:
		return 4
	case zapcore.PanicLevel:
		return 5
	case zapcore.DebugLevel:
		return 6
	default:
		return 0
	}
}
