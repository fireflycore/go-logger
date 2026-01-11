package internal

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewConsoleCore 构造一个输出到 stdout 的 console encoder core。
//
// 设计要点：
// - 采用 ProductionEncoderConfig，字段与 zap 默认生产格式保持一致
// - 通过自定义 EncodeTime/EncodeLevel/EncodeCaller，把输出变成更利于人读的形式
// - 返回的 Core 可与 JSON Core 通过 zapcore.NewTee 合并
func NewConsoleCore(level zapcore.LevelEnabler) zapcore.Core {
	// level 由上层统一构造并传入，确保 Console 与 Remote 共享同一套等级控制逻辑。
	// encoderConfig 决定日志的字段名与编码方式（时间、等级、caller 等）。
	encoderConfig := zap.NewProductionEncoderConfig()
	// ConsoleSeparator 控制 console encoder 的分隔符。
	encoderConfig.ConsoleSeparator = " "
	// MessageKey 与 CallerKey 对齐到本库的字段命名（便于下游统一解析）。
	encoderConfig.MessageKey = "message"
	encoderConfig.CallerKey = "path"
	// 时间统一格式化为可读字符串，并用中括号包裹，便于在终端中快速扫视。
	encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString("[" + t.Format(time.DateTime) + "]")
	}
	// 等级同样用中括号包裹，保持展示对齐。
	encoderConfig.EncodeLevel = func(t zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString("[" + t.String() + "]")
	}
	// caller 使用 TrimmedPath，避免绝对路径过长影响可读性。
	encoderConfig.EncodeCaller = func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + caller.TrimmedPath() + "]")
	}

	// console encoder 面向人读，适合本地调试与开发环境。
	enc := zapcore.NewConsoleEncoder(encoderConfig)
	// 输出目标选择 stdout，并用外部传入的 LevelEnabler 作为等级控制。
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), level)

	return core
}
