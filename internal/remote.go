package internal

import (
	"encoding/json"
	"strings"

	"go.uber.org/zap/zapcore"
)

// ServerLogger 表示服务端日志。
type ServerLogger struct {
	Path    string `json:"path"`
	Level   uint32 `json:"level"`
	Content string `json:"content"`

	TraceId  string `json:"trace_id"`
	ParentId string `json:"parent_id"`

	UserId string `json:"user_id"`
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
	if c.handle == nil {
		return nil
	}

	// 预分配切片，避免多次扩容
	allFields := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	allFields = append(allFields, c.fields...)
	allFields = append(allFields, fields...)

	var traceId, parentId, userId string
	found := 0

	enc := zapcore.NewMapObjectEncoder()
	for _, f := range allFields {
		// 先构建字段映射，确保所有字段都被序列化
		f.AddTo(enc)

		// 提取关注的字段（仅在尚未找到时检查）
		if found < 3 {
			switch {
			case (f.Key == "trace_id" || f.Key == "TraceId") && f.Type == zapcore.StringType && traceId == "":
				traceId = f.String
				found++
			case (f.Key == "user_id" || f.Key == "UserId") && f.Type == zapcore.StringType && userId == "":
				userId = f.String
				found++
			case (f.Key == "parent_id" || f.Key == "ParentId") && f.Type == zapcore.StringType && parentId == "":
				parentId = f.String
				found++
			}
		}
	}

	// 构建 content
	var contentBuilder strings.Builder
	contentBuilder.WriteString(entry.Message)
	if len(enc.Fields) > 0 {
		if b, err := json.Marshal(enc.Fields); err == nil {
			contentBuilder.WriteByte(' ')
			contentBuilder.Write(b)
		}
	}
	content := contentBuilder.String()

	// 序列化最终日志
	b, err := json.Marshal(&ServerLogger{
		Path:     entry.Caller.TrimmedPath(), // TrimmedPath 内部已处理未定义情况
		Level:    levelConvertValue(entry.Level),
		Content:  content,
		TraceId:  traceId,
		ParentId: parentId,
		UserId:   userId,
	})
	if err == nil {
		c.handle(b)
	}
	return nil
}

func (c *remoteCore) Sync() error {
	// handle 是否需要 flush 由 handle 自己保证；此处保持无副作用。
	return nil
}

func levelConvertValue(level zapcore.Level) uint32 {
	// 该映射保持与旧版本一致：下游存储/检索可能依赖数字等级。
	switch level {
	case zapcore.InfoLevel:
		return 1
	case zapcore.WarnLevel:
		return 2
	case zapcore.ErrorLevel:
		return 3
	default:
		return 0
	}
}
