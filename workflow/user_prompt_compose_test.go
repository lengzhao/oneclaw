package workflow

import "testing"

func TestComposeUserPrompt(t *testing.T) {
	tests := []struct {
		meta map[string]any
		base string
		want string
	}{
		{nil, "hello", "hello"},
		{map[string]any{}, "hello", "hello"},
		{
			meta: map[string]any{"user_prompt_suffix": "Translate the above to French."},
			base: "Bonjour",
			want: "Bonjour\n\nTranslate the above to French.",
		},
		{
			meta: map[string]any{"user_prompt_prefix": "Task:"},
			base: "fix bug",
			want: "Task:\n\nfix bug",
		},
		{
			meta: map[string]any{
				"user_prompt_prefix": "You translate carefully.",
				"user_prompt_suffix": "Output only the translation.",
			},
			base: "Hello",
			want: "You translate carefully.\n\nHello\n\nOutput only the translation.",
		},
	}
	for _, tc := range tests {
		got := ComposeUserPrompt(tc.meta, tc.base)
		if got != tc.want {
			t.Fatalf("ComposeUserPrompt(meta=%v, base=%q)=%q want %q", tc.meta, tc.base, got, tc.want)
		}
	}
}
