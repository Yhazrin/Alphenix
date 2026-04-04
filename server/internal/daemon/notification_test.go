package daemon

import (
	"testing"
)

func TestDetectProgress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		want    *ProgressPattern
	}{
		{
			name:  "progress step",
			input: "PROGRESS: step 2/5 — refactoring the handler",
			want:  &ProgressPattern{Current: 2, Total: 5, Phase: "progress", Summary: "refactoring the handler"},
		},
		{
			name:  "progress with em dash",
			input: "PROGRESS: step 1/3 — fixing tests",
			want:  &ProgressPattern{Current: 1, Total: 3, Phase: "progress", Summary: "fixing tests"},
		},
		{
			name:  "phase marker research",
			input: "Research: exploring the codebase",
			want:  &ProgressPattern{Phase: "research", Summary: "exploring the codebase"},
		},
		{
			name:  "phase marker verify",
			input: "Verify: all tests passing",
			want:  &ProgressPattern{Phase: "verify", Summary: "all tests passing"},
		},
		{
			name:  "done header",
			input: "DONE:",
			want:  &ProgressPattern{Phase: "done", Summary: "task completed"},
		},
		{
			name:    "plain text",
			input:   "just some regular output",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectProgress(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if tt.want.Phase != "" && got.Phase != tt.want.Phase {
				t.Errorf("Phase: got %q, want %q", got.Phase, tt.want.Phase)
			}
			if tt.want.Current != 0 && got.Current != tt.want.Current {
				t.Errorf("Current: got %d, want %d", got.Current, tt.want.Current)
			}
			if tt.want.Total != 0 && got.Total != tt.want.Total {
				t.Errorf("Total: got %d, want %d", got.Total, tt.want.Total)
			}
			if tt.want.Summary != "" && got.Summary != tt.want.Summary {
				t.Errorf("Summary: got %q, want %q", got.Summary, tt.want.Summary)
			}
		})
	}
}

func TestClassifyMessage(t *testing.T) {
	tests := []struct {
		msgType string
		want    MessageClass
	}{
		{"error", ClassUrgent},
		{"tool_use", ClassNormal},
		{"tool_result", ClassNormal},
		{"text", ClassBatched},
		{"thinking", ClassBatched},
		{"unknown", ClassBatched},
	}
	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			got := ClassifyMessage(tt.msgType)
			if got != tt.want {
				t.Errorf("ClassifyMessage(%q) = %d, want %d", tt.msgType, got, tt.want)
			}
		})
	}
}
