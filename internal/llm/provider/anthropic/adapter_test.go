package anthropic

import (
	"encoding/json"
	"testing"
)

func TestToolCallsJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "{}",
		},
		{
			name:  "already valid object",
			input: `{"command":"date","description":"get current time"}`,
			want:  `{"command":"date","description":"get current time"}`,
		},
		{
			name:  "already valid array",
			input: `[{"command":"date"}]`,
			want:  `[{"command":"date"}]`,
		},
		{
			name:  "object fragment without braces",
			input: `"command":"date","description":"get current time"`,
			want:  `{"command":"date","description":"get current time"}`,
		},
		{
			name:  "invalid fragment falls back to empty object",
			input: `{"command":"date"`,
			want:  "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolCallsJSON(tt.input)
			if got != tt.want {
				t.Fatalf("toolCallsJSON() = %q, want %q", got, tt.want)
			}
			if !json.Valid([]byte(got)) {
				t.Fatalf("toolCallsJSON() returned invalid JSON: %q", got)
			}
		})
	}
}
