package document

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/editor"
)

func TestExternalReloadSeesAnEditBeforeTheNextUIFrame(t *testing.T) {
	d, h, _ := opened(t, map[string]string{"doc.typ": "original"}, "doc.typ", false)
	// This is the native editor's buffer mutation before View calls sync().
	d.buf.Edit(editor.Edit{Start: 0, End: 0, Text: "pending "})
	if d.unsaved || !d.Unsaved() {
		t.Fatal("test did not reach the pre-frame dirty state")
	}
	if err := os.WriteFile(filepath.Join(h.store.Root(), "doc.typ"), []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.ReloadExternal("external")
	if d.Text() != "pending original" || !d.Unsaved() || d.Status().State != Failed {
		t.Fatal("pre-frame input lost")
	}
	if d.Flush() {
		t.Fatal("pre-frame conflict was overwritten")
	}
}

func TestAnUnchangedDiskNotificationCannotMarkPendingEditsSaved(t *testing.T) {
	d, _, tt := opened(t, map[string]string{"doc.typ": "original"}, "doc.typ", false)
	tt.Type("pending ")
	before := d.Text()
	d.ReloadExternal("original")
	if d.Text() != before || !d.Unsaved() {
		t.Fatal("notification discarded or accepted pending text")
	}
}

func TestExternalReloadPreservesPendingEditsAndStopsAutosave(t *testing.T) {
	d, h, tt := opened(t, map[string]string{"doc.typ": "original"}, "doc.typ", false)
	tt.Type("local ")
	local := d.Text()
	if err := os.WriteFile(filepath.Join(h.store.Root(), "doc.typ"), []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.ReloadExternal("external")
	if d.Text() != local || !d.Unsaved() || d.Status().State != Failed {
		t.Fatal("conflicting buffer not retained")
	}
	time.Sleep(saveDelay + 10*time.Millisecond)
	h.pump()
	if d.Flush() {
		t.Fatal("conflicting save succeeded")
	}
	if text, err := h.store.Load("doc.typ"); err != nil || text != "external" {
		t.Fatalf("disk overwritten: %q %v", text, err)
	}
	// Restoring the expected disk version makes a retry safe.
	if err := os.WriteFile(filepath.Join(h.store.Root(), "doc.typ"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !d.Flush() {
		t.Fatal("safe retry failed")
	}
}

func TestFlushDetectsExternalChangesBeforeTheirNotification(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "delete"}[remove], func(t *testing.T) {
			d, h, tt := opened(t, map[string]string{"doc.typ": "original"}, "doc.typ", false)
			tt.Type("local ")
			path := filepath.Join(h.store.Root(), "doc.typ")
			if remove {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, []byte("external"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if d.Flush() || !d.Unsaved() {
				t.Fatal("save did not protect external change")
			}
			if remove {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("deleted file recreated")
				}
			}
		})
	}
}

func TestExternalReloadAcceptsCleanTextAndOwnSaveEcho(t *testing.T) {
	d, _, tt := opened(t, map[string]string{"doc.typ": "original"}, "doc.typ", false)
	d.ReloadExternal("outside")
	if d.Text() != "outside" || d.Unsaved() {
		t.Fatal("clean text did not reload")
	}
	tt.Type("local ")
	text := d.Text()
	d.ReloadExternal(text)
	if d.Text() != text || d.Unsaved() {
		t.Fatal("own save echo became a conflict")
	}
}
