// Package document is the open document: its editor, its compiles, its
// preview, completion, hover, images, and the way its text reaches disk.
package document

import (
	"errors"
	"io/fs"
	"strings"
	"time"

	"github.com/cheetahbyte/thesio/internal/app/host"
	"github.com/cheetahbyte/thesio/internal/typst"
	"github.com/cheetahbyte/thesio/internal/vault"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const saveDelay = 500 * time.Millisecond

// SaveState is where the text is on its way to disk.
type SaveState int

const (
	// Idle is freshly opened, nothing saved this session.
	Idle SaveState = iota
	// Saved is written this session.
	Saved
	// Saving waits for typing to stop.
	Saving
	// Failed keeps the changes pending; SaveStatus.Error says why.
	Failed
)

// SaveStatus is the save state, with the error of a failed save.
type SaveStatus struct {
	State SaveState
	Error string
}

// Open is a document open in an editor, with its compiles and, unless it is
// paper notes, its preview.
type Open struct {
	// OnChange runs after each edit.
	OnChange func(text string)
	// OnCaret runs when the caret moves.
	OnCaret func(offset int)
	// OnSaved runs after a successful write.
	OnSaved func(id, text string)
	// OnFollowLink runs for a ⌘-click on a note link, with its vault path.
	OnFollowLink func(path string)
	// Citations is what `@` offers, given the query after it and the document's labels.
	Citations func(query string, labels []typst.Label) []editor.Completion
	// NoteLinks is what `[[` offers, given the query after it.
	NoteLinks func(query string) []editor.Completion
	// OnCitationCompleted runs when a citation of a source was accepted.
	OnCitationCompleted func()

	h           host.Host
	id, main    string
	withPreview bool
	buf         *editor.Buffer
	ed          *editor.State
	focus       ui.Handle
	split       float32
	theme       *ui.Theme
	closed      bool

	// seen is the buffer version taken in; caret the caret.
	seen  uint64
	caret int
	frame int

	// diskText is the last version this editor accepted, not a newer external edit.
	diskText   string
	diskExists bool
	unsaved    bool
	status     SaveStatus
	saveGen    uint64

	s  *session
	pv preview

	// typed is the text typed last, which can open completion.
	typed    string
	autoAsk  bool
	offer    *offer
	asking   newest
	hovering newest
	menuAt   int
	// stops are the placeholders Tab reaches next, as distances from the text's end.
	stops [][2]int
	moved bool

	block      blockRange
	blockShown bool
}

// NewOpen opens id with text. main is the top-level document it compiles within;
// withPreview is false for paper notes, which compile for completion and hover only.
func NewOpen(h host.Host, id, text, main string, withPreview bool) *Open {
	d := &Open{h: h, id: id, main: main, withPreview: withPreview, split: 50, diskText: text}
	if _, err := h.Store().Load(id); !errors.Is(err, fs.ErrNotExist) {
		d.diskExists = true
	}
	d.buf = editor.NewBuffer(text)
	d.buf.SetLanguage(typstLanguage)
	d.ed = newEditor(d)
	d.seen = d.buf.Version()
	d.s = newSession(h, h.Store().Root(), withPreview)
	d.refreshBlock(text)
	d.compile(false)
	return d
}

// ID is the document's vault path.
func (d *Open) ID() string { return d.id }

// Text is the editor's text.
func (d *Open) Text() string { return d.buf.String() }

// Caret is the offset of the caret.
func (d *Open) Caret() int { return d.ed.Cursor() }

// Focus gives the editor the keyboard.
func (d *Open) Focus() { d.focus.Focus() }

// GoTo puts the caret at offset and brings it into view.
func (d *Open) GoTo(offset int) {
	offset = max(0, min(offset, d.buf.Len()))
	d.ed.GoTo(d.buf.Pos(offset))
	d.focus.Focus()
}

// Unsaved reports changes not yet on disk.
func (d *Open) Unsaved() bool { return d.unsaved || d.buf.Version() != d.seen }

// Status is the save state, for the header.
func (d *Open) Status() SaveStatus { return d.status }

// Words is the document's words by file, as of the last good compile.
func (d *Open) Words() []typst.FileWords {
	if k := d.s.document; k != nil {
		return k.Words
	}
	return nil
}

// Labels is what `@` can reference, as of the last good compile.
func (d *Open) Labels() []typst.Label {
	if k := d.s.knowledge(); k != nil {
		return k.Labels
	}
	return nil
}

// SetPath follows a rename or move of the document or of its main file.
func (d *Open) SetPath(id, main string) {
	if id == d.id && main == d.main {
		return
	}
	d.id, d.main = id, main
	d.compile(false)
}

// SetText replaces the text: as one undoable edit, or as Thesio's own text
// outside the undo history. The caller writes it to the file; MarkUnsaved and
// MarkSaved tell the outcome.
func (d *Open) SetText(text string, undoable bool) {
	old := d.Text()
	if old == text {
		return
	}
	start, oldEnd, newEnd := difference(old, text)
	caret := d.ed.Cursor()
	switch {
	case caret <= start:
	case caret >= oldEnd:
		caret += newEnd - oldEnd
	default:
		caret = newEnd
	}
	if undoable {
		d.buf.Edit(editor.Edit{Start: start, End: oldEnd, Text: text[start:newEnd]})
	} else {
		d.buf.Reset(text)
	}
	d.selectRange(caret, caret)
	d.seen = d.buf.Version()
	d.offer = nil
	d.refreshBlock(text)
	d.compile(true)
	if undoable && d.OnChange != nil {
		d.OnChange(text)
	}
}

// difference is the span where a and b differ: a[start:aEnd] became b[start:bEnd].
func difference(a, b string) (start, aEnd, bEnd int) {
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	aEnd, bEnd = len(a), len(b)
	for aEnd > start && bEnd > start && a[aEnd-1] == b[bEnd-1] {
		aEnd--
		bEnd--
	}
	return start, aEnd, bEnd
}

// MarkUnsaved makes the text pending, as after text changed without an edit.
func (d *Open) MarkUnsaved() { d.unsaved = true }

// MarkSaved records that the text as it is now reached the file.
func (d *Open) MarkSaved() {
	d.saveGen++
	d.unsaved = false
	d.diskText, d.diskExists = d.Text(), true
	d.status = SaveStatus{State: Saved}
}

// ReloadExternal accepts a disk update only if the editor has no pending edits.
// Even unchanged text recompiles: an include, image or bibliography may differ.
func (d *Open) ReloadExternal(text string) {
	// A UI edit may be in the buffer before the next frame marks it unsaved.
	if d.buf.Version() != d.seen {
		d.edited()
	}
	if d.unsaved && text != d.diskText && text != d.Text() {
		d.externalConflict()
		return
	}
	if !d.unsaved || text == d.Text() {
		d.SetText(text, false)
		d.MarkSaved()
	}
	d.compile(false)
}

func (d *Open) externalConflict() {
	d.saveGen++
	const message = "File changed outside Thesio; your edits are kept here. Copy them before discarding changes and reopening the file"
	if d.status.Error != message {
		d.h.ToastError(message)
	}
	d.status = SaveStatus{State: Failed, Error: message}
}

// ExternalUnavailable prevents a pending save from recreating a removed file.
func (d *Open) ExternalUnavailable(err error) {
	d.saveGen++
	d.status = SaveStatus{State: Failed, Error: "File unavailable outside Thesio; your edits are kept here: " + err.Error()}
}

// Flush writes now if there are unsaved changes; false when that failed, and
// the changes stay pending for the next save to retry.
func (d *Open) Flush() bool {
	if d.buf.Version() != d.seen {
		d.edited()
	}
	d.saveGen++
	if !d.unsaved {
		return true
	}
	text := d.Text()
	// Check before every write as notifications can arrive after autosave's timer.
	current, err := d.h.Store().Load(d.id)
	if err != nil && (!errors.Is(err, fs.ErrNotExist) || d.diskExists) {
		d.ExternalUnavailable(err)
		return false
	}
	if err == nil && current != text && (current != d.diskText || !d.diskExists) {
		d.externalConflict()
		return false
	}
	if err := d.h.Store().Save(d.id, text); err != nil {
		d.status = SaveStatus{State: Failed, Error: "Could not save: " + err.Error()}
		return false
	}
	d.MarkSaved()
	if d.OnSaved != nil {
		d.OnSaved(d.id, text)
	}
	return true
}

// Discard gives unsaved changes up.
func (d *Open) Discard() {
	d.saveGen++
	if d.unsaved {
		d.unsaved = false
		d.status = SaveStatus{State: Idle}
	}
}

// Close stops the compiles and the pending save; Flush first to keep changes.
func (d *Open) Close() {
	d.closed = true
	d.saveGen++
	d.pv.followGen++
	d.s.close()
}

func (d *Open) scheduleSave() {
	d.unsaved = true
	d.status = SaveStatus{State: Saving}
	d.saveGen++
	gen := d.saveGen
	time.AfterFunc(saveDelay, func() {
		d.h.Update(func() {
			if gen == d.saveGen && !d.closed {
				d.Flush()
			}
		})
	})
}

// sync takes in what the editor did since the last frame: edits, caret moves.
func (d *Open) sync() {
	d.frame++
	if v := d.buf.Version(); v != d.seen {
		d.edited()
	}
	d.keepFolded()
	if caret := d.ed.Cursor(); caret != d.caret {
		d.caret = caret
		if !d.moved && d.buf.Version() == d.seen {
			d.stops = nil
		}
		d.moved = false
		if d.OnCaret != nil {
			d.OnCaret(caret)
		}
		d.follow(caret)
	}
}

func (d *Open) edited() {
	d.takeAccepted()
	d.seen = d.buf.Version()
	text := d.Text()
	d.refreshBlock(text)
	d.scheduleSave()
	clear(d.s.tried)
	d.compile(true)
	if d.OnChange != nil {
		d.OnChange(text)
	}
	typed := d.typed
	d.typed = ""
	caret := d.ed.Cursor()
	if strings.HasSuffix(typed, "#") || strings.HasSuffix(typed, "@") || strings.HasSuffix(typed, "[") && strings.HasSuffix(text[:caret], "[[") {
		d.autoAsk = true
		d.ed.Run(editor.ShowCompletions)
		d.autoAsk = false
	}
}

// selectRange selects start to end as Thesio's own move, which keeps the placeholders.
func (d *Open) selectRange(start, end int) {
	d.moved = true
	d.ed.Select(start, end)
}

// replace puts text in place of start to end and the caret after it.
func (d *Open) replace(start, end int, text string) {
	d.buf.Edit(editor.Edit{Start: start, End: end, Text: text})
	at := start + len(text)
	d.selectRange(at, at)
}

// InsertCitation cites key at the caret, spaced off the word before it.
func (d *Open) InsertCitation(key string) {
	sel := d.ed.Selection()
	text := d.Text()
	glue := ""
	if before := lastRune(text[:sel.Start()]); before != 0 && !isSpace(before) && !strings.ContainsRune("([{", before) {
		glue = " "
	}
	d.replace(sel.Start(), sel.End(), glue+"@"+key)
}

// InsertQuote puts a quotation at the caret: inline as a citation, or a block
// in a paragraph of its own.
func (d *Open) InsertQuote(markup string, block bool) {
	sel := d.ed.Selection()
	text := d.Text()
	before := text[:sel.Start()]
	var insert string
	if block {
		insert = vault.BlockGlue(before) + markup + "\n\n"
	} else {
		glue, tail := "", ""
		if r := lastRune(before); r != 0 && !isSpace(r) && !strings.ContainsRune("([{", r) {
			glue = " "
		}
		if r := firstRune(text[sel.End():]); r != 0 && isAlnum(r) {
			tail = " "
		}
		insert = glue + markup + tail
	}
	d.replace(sel.Start(), sel.End(), insert)
}
