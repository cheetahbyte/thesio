package vault

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListSeesSameSizeEditsWithinASecond(t *testing.T) {
	root := t.TempDir()
	store, _, err := OpenAt(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "external.typ")
	write := func(text string, nanos int64) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(1800000000, nanos)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	write("before", 100000000)
	if got := store.List()[0].Body; got != "before" {
		t.Fatal(got)
	}
	write("after!", 200000000)
	if got := store.List()[0].Body; got != "after!" {
		t.Fatalf("stale cached text %q", got)
	}
	// A notification must also invalidate a timestamp-preserving write.
	write("newest", 200000000)
	store.InvalidateBodies()
	if got := store.List()[0].Body; got != "newest" {
		t.Fatalf("stale notified text %q", got)
	}
}
