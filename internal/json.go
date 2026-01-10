package internal

import (
	"time"

	"github.com/fireflycore/go-logger/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewJsonCore 构造一个 JSON encoder core，并把输出写到 config。
//
// 典型用途：
// - 将日志写入自定义回调（例如上报到日志服务）
// - 将日志写入异步队列（config 可实现为 AsyncLogger）
//
// 注意：本 core 仅负责 JSON 编码与写入，不做采样/过滤等策略。
func NewJsonCore(config core.LoggerConfig) zapcore.Core {
	// 采用 ProductionEncoderConfig，输出字段更稳定，适合机器解析。
	encoderConfig := zap.NewProductionEncoderConfig()
	// 时间字段写入 created_at，保持与本库 Conf.Write 的解析字段一致。
	encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Format(time.DateTime))
	}
	encoderConfig.MessageKey = "message"
	encoderConfig.CallerKey = "path"
	encoderConfig.TimeKey = "created_at"

	// JSON encoder 面向机器解析，适合远端日志系统。
	enc := zapcore.NewJSONEncoder(encoderConfig)
	// 将用户提供的 LoggerConfig 适配为 zap 的 WriteSyncer。
	writeSync := zapcore.AddSync(config)

	return zapcore.NewCore(enc, writeSync, zap.NewAtomicLevel())
}
