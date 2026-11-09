package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/cheetahbyte/thesio/internal/app/host"
	"github.com/cheetahbyte/thesio/internal/vault"
	"github.com/egoist/mygo"
)

// watchFiles belongs to this window, not the process: a reopened window gets
// a fresh snapshot, and callbacks queued by a closed one do nothing.
func (ws *Workspace) watchFiles() {
	watcher, err := mygo.WatchFiles(context.Background(), ws.store.Root(), mygo.FileWatchOptions{
		Recursive: true,
		Ignore: func(path string, _ bool) bool {
			for _, part := range strings.Split(path, "/") {
				if strings.HasPrefix(part, ".") {
					return true
				}
			}
			return false
		},
	})
	if err != nil {
		ws.ToastError(fmt.Sprintf("Could not watch the vault: %v", err))
		return
	}
	ws.stopWatching = func() {
		if err := watcher.Close(); err != nil {
			ws.ToastError(fmt.Sprintf("Could not stop watching the vault: %v", err))
		}
	}
	// Enrollment and the initial listing must cover the same files.
	ws.RefreshEntries()
	go func() {
		for {
			events, err := watcher.Next(context.Background())
			if err != nil {
				if !errors.Is(err, io.EOF) {
					ws.Update(func() {
						if !ws.closed {
							ws.ToastError(fmt.Sprintf("Vault watching stopped: %v", err))
						}
					})
				}
				return
			}
			ws.queueFileChanges(events)
		}
	}()
}

func (ws *Workspace) queueFileChanges(events []mygo.FileEvent) {
	ws.Update(func() {
		if !ws.closed {
			ws.filesChanged(events)
		}
	})
}

// Close releases the window's subscription and pending document work.
func (ws *Workspace) Close() {
	if ws.closed {
		return
	}
	ws.closed = true
	if ws.stopWatching != nil {
		ws.stopWatching()
		ws.stopWatching = nil
	}
	if ws.doc != nil {
		ws.doc.Close()
	}
	ws.papers.ClosePaper()
}

// filesChanged treats events as hints and reconciles with disk, including a
// whole directory arriving or going away. It never writes external changes back.
func (ws *Workspace) filesChanged(events []mygo.FileEvent) {
	for _, e := range events {
		if e.Op == mygo.FileRename {
			ws.PathChanged(e.OldPath, e.Path, nil)
		}
	}
	ws.store.InvalidateBodies()
	ws.RefreshEntries()
	for _, e := range events {
		if e.Path == vault.LibraryFile || e.OldPath == vault.LibraryFile {
			ws.ReloadLibrary()
			ws.cites.TableChanged()
			break
		}
	}
	editors := ws.editorsOf(ws.Current())
	if notes := ws.notesID(); notes != "" && notes != ws.Current() {
		editors = append(editors, ws.editorsOf(notes)...)
	}
	for _, d := range editors {
		text, err := ws.store.Load(d.ID())
		if err != nil {
			if d.Unsaved() {
				d.ExternalUnavailable(err)
				continue
			}
			if errors.Is(err, fs.ErrNotExist) && d == ws.doc {
				d.Close()
				ws.doc = nil
				if ws.view == host.ViewDoc {
					ws.setView(host.ViewHome)
				}
			} else {
				ws.ToastError(fmt.Sprintf("Could not reload %s: %v", d.ID(), err))
			}
			continue
		}
		d.ReloadExternal(text)
	}
	ws.followTopDocument()
	ws.refreshOutline()
	ws.Invalidate()
}
