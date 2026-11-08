package logger

import (
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	// 通过多组配置覆盖：
	// - 仅控制台
	// - 仅远端（需要 handle）
	// - 同时启用
	// - 都不启用（应返回 nop，不触发 handle）
	// - 远端启用但未提供 handle（不应触发 handle）
	tests := []struct {
		name        string
		config      *Conf
		wantConsole bool
		wantRemote  bool
		withHandle  bool
	}{
		{
			name: "console only",
			config: &Conf{
				Console: true,
				Remote:  false,
			},
			wantConsole: true,
			wantRemote:  false,
			withHandle:  true,
		},
		{
			name: "remote only",
			config: &Conf{
				Console: false,
				Remote:  true,
			},
			wantConsole: false,
			wantRemote:  true,
			withHandle:  true,
		},
		{
			name: "both console and remote",
			config: &Conf{
				Console: true,
				Remote:  true,
			},
			wantConsole: true,
			wantRemote:  true,
			withHandle:  true,
		},
		{
			name: "neither console nor remote",
			config: &Conf{
				Console: false,
				Remote:  false,
			},
			wantConsole: false,
			wantRemote:  false,
			withHandle:  true,
		},
		{
			name: "remote enabled but no handle",
			config: &Conf{
				Console: false,
				Remote:  true,
			},
			wantConsole: false,
			wantRemote:  true,
			withHandle:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 记录是否触发回调以及最后一次回调内容。
			handleCalled := 0
			var last []byte
			handle := func(b []byte) {
				handleCalled++
				last = append(last[:0], b...)
			}

			// 按用例决定是否提供 handle，模拟真实调用场景。
			var logger *zap.Logger
			if tt.withHandle {
				logger = New(tt.config, handle)
			} else {
				logger = New(tt.config, nil)
			}

			// 写一条日志用于触发输出。
			logger.Info("测试", zap.Bool("cancel", true))

			// Remote + handle：要求回调被触发，且输出可解析为 JSON 并包含关键字段。
			if tt.config.Remote && tt.withHandle {
				if handleCalled == 0 {
					t.Fatalf("expected handle to be called")
				}
				var m map[string]any
				if err := json.Unmarshal(last, &m); err != nil {
					t.Fatalf("expected json output, got error: %v", err)
				}
				if _, ok := m["path"]; !ok {
					t.Fatalf("expected path field")
				}
				if _, ok := m["level"]; !ok {
					t.Fatalf("expected level field")
				}
				content, ok := m["content"].(string)
				if !ok || !strings.HasPrefix(content, "测试 ") || !strings.Contains(content, `"cancel":true`) {
					t.Fatalf("unexpected content: %v", m["content"])
				}
				if m["level"] != float64(1) {
					t.Fatalf("unexpected level: %v", m["level"])
				}
			} else {
				// 其他情况：不应触发 handle（例如只开 console、未开任何输出、未提供 handle 等）。
				if handleCalled != 0 {
					t.Fatalf("expected handle not to be called")
				}
			}
		})
	}
}
