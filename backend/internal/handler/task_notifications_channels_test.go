package handler

import (
	"testing"

	"project-manager/backend/internal/config"
)

func TestResolveNonEmailTaskProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		want    string
		wantErr bool
	}{
		{
			name: "auto single wecom",
			cfg: config.Config{
				WeComCorpID:     "corp",
				WeComCorpSecret: "secret",
				WeComAgentID:    "1000001",
			},
			want: "wecom",
		},
		{
			name: "auto conflict",
			cfg: config.Config{
				WeComCorpID:     "corp",
				WeComCorpSecret: "secret",
				WeComAgentID:    "1000001",
				DingTalkWebhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
			},
			wantErr: true,
		},
		{
			name: "explicit dingtalk with config",
			cfg: config.Config{
				TaskNotifyProvider: "dingtalk",
				DingTalkWebhook:    "https://oapi.dingtalk.com/robot/send?access_token=xxx",
			},
			want: "dingtalk",
		},
		{
			name: "explicit feishu but not configured",
			cfg: config.Config{
				TaskNotifyProvider: "feishu",
			},
			want: "",
		},
		{
			name: "explicit none",
			cfg: config.Config{
				TaskNotifyProvider: "none",
				DingTalkWebhook:    "https://oapi.dingtalk.com/robot/send?access_token=xxx",
			},
			want: "",
		},
		{
			name: "invalid provider",
			cfg: config.Config{
				TaskNotifyProvider: "unknown",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			h := &Handler{Cfg: test.cfg}
			got, err := h.resolveNonEmailTaskProvider()
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%s)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("provider mismatch: got=%s want=%s", got, test.want)
			}
		})
	}
}
