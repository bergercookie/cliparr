package api

import (
	"testing"
	"time"
)

func TestPendingQuickConnectsExpire(t *testing.T) {
	p := newPendingQuickConnects(20*time.Millisecond, 10)
	id, ok := p.add("secret")
	if !ok {
		t.Fatal("add refused")
	}
	if secret, found := p.secret(id); !found || secret != "secret" {
		t.Fatalf("fresh entry: %q, %v", secret, found)
	}
	time.Sleep(40 * time.Millisecond)
	if _, found := p.secret(id); found {
		t.Error("an expired sign-in must be gone")
	}
}

func TestPendingQuickConnectsFreeRoomWhenEntriesExpire(t *testing.T) {
	p := newPendingQuickConnects(20*time.Millisecond, 1)
	if _, ok := p.add("one"); !ok {
		t.Fatal("first add refused")
	}
	if _, ok := p.add("two"); ok {
		t.Fatal("the cap should refuse while one waits")
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok := p.add("three"); !ok {
		t.Error("an expired entry should make room again")
	}
}

func TestPendingQuickConnectsGiveUnguessableIDs(t *testing.T) {
	p := newPendingQuickConnects(time.Minute, 10)
	a, _ := p.add("x")
	b, _ := p.add("x")
	if a == b || len(a) != 32 {
		t.Errorf("ids should be distinct 128-bit values, got %q and %q", a, b)
	}
}
