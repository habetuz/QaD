package storage

import "testing"

// --- NoEvictionStorage ---

func TestNoEvictionStorage_ReadWrite(t *testing.T) {
	s := NewNoEvictionStorage()
	s.Write("key", []byte("value"))
	if got := string(s.Read("key")); got != "value" {
		t.Errorf("expected %q, got %q", "value", got)
	}
}

func TestNoEvictionStorage_ReadMissing(t *testing.T) {
	s := NewNoEvictionStorage()
	if s.Read("missing") != nil {
		t.Error("expected nil for missing key")
	}
}

func TestNoEvictionStorage_Delete(t *testing.T) {
	s := NewNoEvictionStorage()
	s.Write("key", []byte("value"))
	s.Delete("key")
	if s.Read("key") != nil {
		t.Error("expected nil after delete")
	}
}

func TestNoEvictionStorage_DeleteMissing(t *testing.T) {
	s := NewNoEvictionStorage()
	s.Delete("nonexistent") // must not panic
}

func TestNoEvictionStorage_Overwrite(t *testing.T) {
	s := NewNoEvictionStorage()
	s.Write("key", []byte("old"))
	s.Write("key", []byte("new"))
	if got := string(s.Read("key")); got != "new" {
		t.Errorf("expected %q, got %q", "new", got)
	}
}

func TestNoEvictionStorage_MultipleKeys(t *testing.T) {
	s := NewNoEvictionStorage()
	s.Write("a", []byte("1"))
	s.Write("b", []byte("2"))
	s.Write("c", []byte("3"))
	for key, want := range map[string]string{"a": "1", "b": "2", "c": "3"} {
		if got := string(s.Read(key)); got != want {
			t.Errorf("key %q: expected %q, got %q", key, want, got)
		}
	}
}

// --- FIFOStorage ---

func TestFIFOStorage_ReadWrite(t *testing.T) {
	s := NewFIFOStorage(1024)
	s.Write("key", []byte("value"))
	if got := string(s.Read("key")); got != "value" {
		t.Errorf("expected %q, got %q", "value", got)
	}
}

func TestFIFOStorage_ReadMissing(t *testing.T) {
	s := NewFIFOStorage(1024)
	if s.Read("missing") != nil {
		t.Error("expected nil for missing key")
	}
}

func TestFIFOStorage_Delete(t *testing.T) {
	s := NewFIFOStorage(1024)
	s.Write("key", []byte("value"))
	s.Delete("key")
	if s.Read("key") != nil {
		t.Error("expected nil after delete")
	}
}

func TestFIFOStorage_DeleteMissing(t *testing.T) {
	s := NewFIFOStorage(1024)
	s.Delete("nonexistent") // must not panic
}

func TestFIFOStorage_EvictionOrder(t *testing.T) {
	// maxSize=6: fits two 3-byte values; writing a third evicts the oldest.
	s := NewFIFOStorage(6)
	s.Write("first", []byte("aaa"))
	s.Write("second", []byte("bbb"))
	s.Write("third", []byte("ccc")) // "first" should be evicted

	if s.Read("first") != nil {
		t.Error("expected 'first' to be evicted (oldest)")
	}
	if s.Read("second") == nil {
		t.Error("expected 'second' to survive")
	}
	if s.Read("third") == nil {
		t.Error("expected 'third' to be present")
	}
}

func TestFIFOStorage_MultipleEvictions(t *testing.T) {
	s := NewFIFOStorage(9) // fits three 3-byte values
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Write("c", []byte("ccc"))
	// Writing 9 bytes at once evicts all three.
	s.Write("d", []byte("ddddddddd"))

	for _, key := range []string{"a", "b", "c"} {
		if s.Read(key) != nil {
			t.Errorf("expected %q to be evicted", key)
		}
	}
	if s.Read("d") == nil {
		t.Error("expected 'd' to be present")
	}
}

func TestFIFOStorage_UpdateExistingKeyNoEviction(t *testing.T) {
	// Updating an existing key with the same size must not evict any key.
	s := NewFIFOStorage(6)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Write("a", []byte("xxx")) // same size, update in place

	if got := string(s.Read("a")); got != "xxx" {
		t.Errorf("expected updated value %q, got %q", "xxx", got)
	}
	if s.Read("b") == nil {
		t.Error("expected 'b' to survive after same-size update of 'a'")
	}
}

func TestFIFOStorage_UpdateGrowthTriggersEviction(t *testing.T) {
	// Growing a key's value so total exceeds maxSize triggers eviction.
	s := NewFIFOStorage(6)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Write("a", []byte("aaaaaa")) // grows to 6 bytes; "b" must be evicted

	if s.Read("b") != nil {
		t.Error("expected 'b' to be evicted after 'a' grew beyond limit")
	}
	if got := string(s.Read("a")); got != "aaaaaa" {
		t.Errorf("expected new value %q, got %q", "aaaaaa", got)
	}
}

func TestFIFOStorage_DeleteFreesSpace(t *testing.T) {
	s := NewFIFOStorage(6)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Delete("a")               // frees 3 bytes
	s.Write("c", []byte("ccc")) // should fit without evicting "b"

	if s.Read("b") == nil {
		t.Error("expected 'b' to survive")
	}
	if s.Read("c") == nil {
		t.Error("expected 'c' to be present")
	}
}

// --- LRUStorage ---

func TestLRUStorage_ReadWrite(t *testing.T) {
	s := NewLRUStorage(1024)
	s.Write("key", []byte("value"))
	if got := string(s.Read("key")); got != "value" {
		t.Errorf("expected %q, got %q", "value", got)
	}
}

func TestLRUStorage_ReadMissing(t *testing.T) {
	s := NewLRUStorage(1024)
	if s.Read("missing") != nil {
		t.Error("expected nil for missing key")
	}
}

func TestLRUStorage_Delete(t *testing.T) {
	s := NewLRUStorage(1024)
	s.Write("key", []byte("value"))
	s.Delete("key")
	if s.Read("key") != nil {
		t.Error("expected nil after delete")
	}
}

func TestLRUStorage_DeleteMissing(t *testing.T) {
	s := NewLRUStorage(1024)
	s.Delete("nonexistent") // must not panic
}

func TestLRUStorage_EvictsLeastRecentlyWritten(t *testing.T) {
	// With no reads in between, eviction order is determined by write order:
	// the first-written key is least recently used.
	s := NewLRUStorage(9)       // fits three 3-byte values
	s.Write("a", []byte("aaa")) // LRU
	s.Write("b", []byte("bbb"))
	s.Write("c", []byte("ccc")) // MRU
	s.Write("d", []byte("ddd")) // "a" evicted

	if s.Read("a") != nil {
		t.Error("expected 'a' to be evicted (LRU)")
	}
	if s.Read("b") == nil {
		t.Error("expected 'b' to survive")
	}
	if s.Read("c") == nil {
		t.Error("expected 'c' to survive")
	}
	if s.Read("d") == nil {
		t.Error("expected 'd' to be present")
	}
}

func TestLRUStorage_UpdatePromotesToFront(t *testing.T) {
	// Writing an existing key promotes it to MRU.
	// After: a=MRU, c=middle, b=LRU -> writing "d" evicts "b".
	s := NewLRUStorage(9)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Write("c", []byte("ccc"))
	s.Write("a", []byte("xxx")) // promotes "a"; "b" becomes LRU
	s.Write("d", []byte("ddd")) // evicts "b"

	if s.Read("b") != nil {
		t.Error("expected 'b' to be evicted (LRU after 'a' was promoted)")
	}
	if got := string(s.Read("a")); got != "xxx" {
		t.Errorf("expected updated value %q, got %q", "xxx", got)
	}
}

func TestLRUStorage_UpdateExistingKeyNoEviction(t *testing.T) {
	s := NewLRUStorage(6)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Write("a", []byte("xxx"))

	if got := string(s.Read("a")); got != "xxx" {
		t.Errorf("expected %q, got %q", "xxx", got)
	}
	if s.Read("b") == nil {
		t.Error("expected 'b' to survive")
	}
}

func TestLRUStorage_DeleteFreesSpace(t *testing.T) {
	s := NewLRUStorage(6)
	s.Write("a", []byte("aaa"))
	s.Write("b", []byte("bbb"))
	s.Delete("a")
	s.Write("c", []byte("ccc"))

	if s.Read("b") == nil {
		t.Error("expected 'b' to survive")
	}
	if s.Read("c") == nil {
		t.Error("expected 'c' to be present")
	}
}
