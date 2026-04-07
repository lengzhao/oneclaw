package openaistub

import "testing"

func TestChatRequestUserTextConcat_stringContent(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"system","content":"s"},{"role":"user","content":"hello"},{"role":"user","content":"world"}]}`)
	s, err := ChatRequestUserTextConcat(body)
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello\nworld" {
		t.Fatalf("got %q", s)
	}
}

func TestChatRequestSystemTextConcat(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"system","content":"alpha"},{"role":"user","content":"u"},{"role":"system","content":"beta"}]}`)
	s, err := ChatRequestSystemTextConcat(body)
	if err != nil {
		t.Fatal(err)
	}
	if s != "alpha\nbeta" {
		t.Fatalf("got %q", s)
	}
}

func TestFirstChatUserMessageContaining(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"a"},{"role":"user","content":"needle here"}]}`)
	got, ok, err := FirstChatUserMessageContaining(body, "needle")
	if err != nil || !ok || got != "needle here" {
		t.Fatalf("ok=%v err=%v got=%q", ok, err, got)
	}
}
