package session

import (
	"testing"
	"time"
)

func TestStore_CreateGetDelete(t *testing.T) {
	store := NewStore(24 * time.Hour)
	sid, err := store.Create("token123", "user1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sid == "" {
		t.Fatal("Create returned empty session ID")
	}
	data := store.Get(sid)
	if data == nil {
		t.Fatal("Get returned nil")
	}
	if data.UserName != "user1" || data.SaujanaToken != "token123" {
		t.Errorf("Get: got %+v", data)
	}
	store.Delete(sid)
	data = store.Get(sid)
	if data != nil {
		t.Error("Get after Delete should return nil")
	}
}

func TestStore_Expiry(t *testing.T) {
	store := NewStore(10 * time.Millisecond)
	sid, err := store.Create("t", "u")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	time.Sleep(15 * time.Millisecond)
	data := store.Get(sid)
	if data != nil {
		t.Error("Get after TTL should return nil")
	}
}
