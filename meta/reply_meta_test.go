package meta

import (
	"reflect"
	"testing"
)

func TestFilterReplyMetaByKeys(t *testing.T) {
	m := map[string]string{
		"context_token": "t1",
		"noise":         "x",
		"empty":         "  ",
	}
	got := FilterReplyMetaByKeys(m, []string{"context_token", "empty", "missing"})
	want := map[string]string{"context_token": "t1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if FilterReplyMetaByKeys(m, nil) != nil {
		t.Fatal("nil keys")
	}
	if FilterReplyMetaByKeys(nil, []string{"a"}) != nil {
		t.Fatal("nil map")
	}
}
