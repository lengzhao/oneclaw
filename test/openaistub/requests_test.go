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
