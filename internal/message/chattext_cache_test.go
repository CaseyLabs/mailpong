package message

import (
	"fmt"
	"reflect"
	"testing"
)

func TestChatTextCache(t *testing.T) {
	c := NewChatTextCache()

	first := c.Extract("m1", "hello", "")
	if first.Text != "hello" {
		t.Fatalf("Text = %q, want %q", first.Text, "hello")
	}
	if got := c.Extract("m1", "hello", ""); !reflect.DeepEqual(got, first) {
		t.Errorf("cached result = %+v, want %+v", got, first)
	}
	if got := c.Extract("m1", "changed", ""); got.Text != "changed" {
		t.Errorf("after body change Text = %q, want %q", got.Text, "changed")
	}

	for i := range chatTextCacheSize + 10 {
		c.Extract(fmt.Sprintf("id%d", i), "x", "")
	}
	if n := len(c.entries); n > chatTextCacheSize {
		t.Errorf("cache holds %d entries, want at most %d", n, chatTextCacheSize)
	}
}
