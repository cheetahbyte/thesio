package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cheetahbyte/thesio/internal/app/host"
	"github.com/egoist/mygo"
)

func TestExternalChangesRefreshTheSidebarAndSearch(t *testing.T) {
	ws, tt := workspace(t)
	t.Cleanup(ws.Close)
	root := ws.Store().Root()
	write := func(id, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, id), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	notify := func(events ...mygo.FileEvent) {
		ws.queueFileChanges(events)
		tt.Frame()
	}
	write("External.typ", "= Outside")
	notify(mygo.FileEvent{Op: mygo.FileCreate, Path: "External.typ"})
	if !tt.HasText("External") {
		t.Fatal("externally created file missing from sidebar")
	}
	info, err := os.Stat(filepath.Join(root, "External.typ"))
	if err != nil {
		t.Fatal(err)
	}
	write("External.typ", "= Changed")
	if err := os.Chtimes(filepath.Join(root, "External.typ"), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	notify(mygo.FileEvent{Op: mygo.FileWrite, Path: "External.typ"})
	if e, _ := ws.Entry("External.typ"); e.Body != "= Changed" {
		t.Fatalf("stale search body %q", e.Body)
	}
	if err := os.MkdirAll(filepath.Join(root, "Folder", "Nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	write("Folder/Nested/Child.typ", "= Child")
	notify(mygo.FileEvent{Op: mygo.FileCreate, Path: "Folder", IsDir: true})
	if _, ok := ws.Entry("Folder/Nested/Child.typ"); !ok {
		t.Fatal("nested file missing")
	}
	if err := os.Rename(filepath.Join(root, "Folder"), filepath.Join(root, "Moved")); err != nil {
		t.Fatal(err)
	}
	notify(mygo.FileEvent{Op: mygo.FileRename, OldPath: "Folder", Path: "Moved", IsDir: true})
	if _, ok := ws.Entry("Moved/Nested/Child.typ"); !ok {
		t.Fatal("renamed directory missing")
	}
	if err := os.RemoveAll(filepath.Join(root, "Moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "External.typ")); err != nil {
		t.Fatal(err)
	}
	notify(mygo.FileEvent{Op: mygo.FileRemove, Path: "Moved", IsDir: true}, mygo.FileEvent{Op: mygo.FileRemove, Path: "External.typ"})
	if len(ws.Entries()) != 0 {
		t.Fatalf("deleted entries remain: %v", ws.Entries())
	}
}

func TestExternalAtomicSaveReloadsAndDeletionLeavesHome(t *testing.T) {
	ws, tt := workspace(t)
	t.Cleanup(ws.Close)
	id, err := ws.Store().CreateFrom("", "Draft", "= Before")
	if err != nil {
		t.Fatal(err)
	}
	ws.RefreshEntries()
	ws.Open(id)
	tt.Frame()
	path := filepath.Join(ws.Store().Root(), id)
	if err := os.WriteFile(path+".tmp", []byte("= After"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
	ws.filesChanged([]mygo.FileEvent{{Op: mygo.FileWrite, Path: id}})
	if ws.doc.Text() != "= After" || ws.doc.Unsaved() {
		t.Fatal("clean editor did not reload")
	}
	if err := os.Rename(path, filepath.Join(ws.Store().Root(), "Renamed.typ")); err != nil {
		t.Fatal(err)
	}
	ws.filesChanged([]mygo.FileEvent{{Op: mygo.FileRename, OldPath: id, Path: "Renamed.typ"}})
	if ws.Current() != "Renamed.typ" {
		t.Fatalf("open document did not follow rename: %s", ws.Current())
	}
	if err := os.Remove(filepath.Join(ws.Store().Root(), "Renamed.typ")); err != nil {
		t.Fatal(err)
	}
	ws.filesChanged([]mygo.FileEvent{{Op: mygo.FileRemove, Path: "Renamed.typ"}})
	if ws.Current() != "" || ws.CurrentView() != host.ViewHome {
		t.Fatal("deleted document remained open")
	}
}

func TestQueuedChangesFromAClosedWorkspaceCannotReachAnotherRoot(t *testing.T) {
	old, oldView := workspace(t)
	next, nextView := workspace(t)
	t.Cleanup(next.Close)
	if err := os.WriteFile(filepath.Join(old.Store().Root(), "Old.typ"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(next.Store().Root(), "Next.typ"), []byte("next"), 0o600); err != nil {
		t.Fatal(err)
	}
	old.queueFileChanges([]mygo.FileEvent{{Op: mygo.FileCreate, Path: "Old.typ"}})
	old.Close()
	next.queueFileChanges([]mygo.FileEvent{{Op: mygo.FileCreate, Path: "Next.typ"}})
	oldView.Frame()
	nextView.Frame()
	if len(old.Entries()) != 0 {
		t.Fatal("closed workspace applied a stale callback")
	}
	if _, ok := next.Entry("Next.typ"); !ok {
		t.Fatal("replacement workspace missed its own root")
	}
	if _, ok := next.Entry("Old.typ"); ok {
		t.Fatal("replacement workspace read the old root")
	}
}

func TestWorkspaceCloseIsIdempotent(t *testing.T) {
	ws, _ := workspace(t)
	stops := 0
	ws.stopWatching = func() { stops++ }
	ws.Close()
	ws.Close()
	if stops != 1 || !ws.closed {
		t.Fatalf("subscription stopped %d times", stops)
	}
}
