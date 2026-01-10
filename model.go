package logger

// levelToInt 将 zap 输出的字符串等级映射为整型等级。
//
// 该映射用于保持历史兼容：外部系统可能依赖 Level 为数字而非字符串。
func levelToInt(level string) int {
	switch level {
	case "info":
		return 1
	case "warn":
		return 3
	case "error":
		return 4
	case "panic":
		return 5
	case "debug":
		return 6
	default:
		return 0
	}
}

// zapLog 对应 zap JSON encoder 的输出结构（本库只解析关心的字段）。
type zapLog struct {
	Level     string `json:"level"`
	CreatedAt string `json:"created_at"`
	Path      string `json:"path"`
	Message   string `json:"message"`
	TraceId   string `json:"trace_id"`
}

// outputLog 为回调 handle 的输出结构。
//
// 字段名保持首字母大写，兼容既有的下游解析与存量数据格式。
type outputLog struct {
	Path      string `json:"Path"`
	Level     int    `json:"Level"`
	Content   string `json:"Content"`
	TraceId   string `json:"TraceId"`
	CreatedAt string `json:"CreatedAt"`
}
