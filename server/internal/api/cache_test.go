package api

import (
	"testing"
	"time"
)

func TestTTLCacheExpiresEntries(t *testing.T) {
	c := newTTLCache[string](20 * time.Millisecond)
	c.set("k", "v")
	if v, ok := c.get("k"); !ok || v != "v" {
		t.Fatalf("fresh entry missing: %q, %v", v, ok)
	}

	time.Sleep(40 * time.Millisecond)
	if _, ok := c.get("k"); ok {
		t.Error("entry outlived its TTL")
	}
}

func TestTTLCacheStaysBounded(t *testing.T) {
	c := newTTLCache[int](time.Hour)
	for i := 0; i < maxCacheEntries+10; i++ {
		c.set(string(rune('a'+i%26))+time.Duration(i).String(), i)
	}
	if n := len(c.entries); n > maxCacheEntries {
		t.Errorf("cache holds %d entries, limit is %d", n, maxCacheEntries)
	}
}
