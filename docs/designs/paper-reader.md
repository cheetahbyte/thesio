# Paper reader

**Status:** abandoned

**Outcome:** built and then removed at the user's decision after trying it. With a real mouse the PDF.js viewer felt worse than the web view's own: dragging to select was slow, the pane flashed or glitched when the paper was read again after a highlight, and it did not look or feel native. The built-in viewer with copy-then-highlight stays. What follows describes what was built, as a record. The code is not in the repository.

## Problem

A paper opens in the web view's built-in PDF viewer, which the app cannot see into. Three things follow for someone reading a source:

- To highlight, they select text, copy it, then press the highlighter. The app searches the paper for the copied text and highlights its first occurrence, which is the wrong place when the phrase appears earlier, and fails when the text breaks across a hyphenated line.
- A highlight card in the panel goes to the top of the highlight's page, not to the highlight. Clicking a highlight in the paper does nothing. A search result for a highlight opens the paper at its start.
- Off macOS the viewer depends on what the platform's web view does with a PDF file.

## Solution

The paper is drawn by PDF.js, shipped inside the app, in the web view the Paper view already has. The app can then learn where the selection is, so Highlight Selection highlights what is selected with no copy step, and it can scroll to a highlight and hear a click on one. Highlights stay standard annotations inside the PDF, written through PDFKit on macOS.

This is the first slice of a better reader: the new viewer, exact highlights on macOS, and navigation both ways between the paper and the highlight cards.

## Behavior

1. **Opening a paper.** The paper shows fitted to the width of the pane, at its first page, with selectable text. Pages render as they come into view.
2. **Highlighting.** With text selected in the paper, choosing Highlight from the right-click menu, or File > Highlight Selection (⌃⌘H), highlights the selected text where it is, however often the phrase occurs and across line breaks and hyphens. The panel opens on Highlights, the new card is there and marked as selected, and the paper stays where it was scrolled. The clipboard is not read or changed.
3. **A selection over several pages** makes one highlight per page, each with its own card. They are written together: either all are in the file or none is.
4. **Highlighting the same text again.** If the selection gives a highlight with the same page and bounds as an existing one, nothing is written and the existing card is marked as selected.
5. **Nothing selected.** Highlight shows "Select some text in the paper first". The same happens in a scanned paper with no text, where nothing can be selected.
6. **A failed write** (the file is read-only, PDFKit refuses it) shows "Could not save the highlight: …" as today, and the paper is unchanged.
7. **Card to paper.** Clicking a card's text scrolls the paper so the highlight is in view and marks it briefly.
8. **Paper to card.** Clicking a highlight in the paper opens the panel on Highlights, scrolls its card into view and marks it as selected. A click that arrives while the highlight list is being read again is answered once the list is read. A click that matches no card does nothing.
9. **Search.** Choosing a highlight in the search panel opens its paper with that highlight in view and its card marked as selected, also when unsaved work has to be saved first.
10. **Comments and deletion** work from the card as today. After either, the paper shows the new state without losing its scroll position. The selected card stays selected if its highlight still exists, and no card is selected otherwise.
11. **Highlights made elsewhere.** A highlight annotation written by another program shows in the paper and in the panel, and behaves like one made here. Other annotation kinds are drawn as PDF.js draws them and have no card, as today.
12. **Zoom.** Pinch, ⌘+ and ⌘− zoom the paper while the viewer has the keyboard; ⌘0 returns to fit width. Pages are drawn again at the new size, so text stays sharp, and highlights stay on their text.
13. **Links.** A link inside the paper to another place in it (a citation, a section) scrolls there. A link to an `http`, `https` or `mailto` address opens in the system's browser or mail program; other addresses do nothing. The viewer never navigates away from the paper.
14. **The file changes while open**, by a highlight written here or by another program: the viewer shows the new file at the same scroll position. If the new file cannot be read, as while another program is still writing it, the viewer keeps showing what it had and tries again at the next change.
15. **The file is renamed or moved.** The paper stays open at its new path and scroll position.
16. **A paper that cannot be shown.** If PDF.js cannot open the file (damaged, or encrypted with a password), or the viewer page does not start, the pane shows the existing message "This PDF could not be shown here. Try opening it in your default app." in place of the viewer. Opening another paper, or a later change to the file that can be read, clears it.
17. **Leaving the paper** closes the document in the viewer, so its memory is released while another view shows.
18. **The viewer's process ends** (the system ends web content processes under memory pressure): the viewer starts again and shows the paper at its first page.
19. **Off macOS** the paper shows and can be read, selected, zoomed and navigated, and highlights already in the file are drawn. The right-click menu has no Highlight item there, File > Highlight Selection does nothing, the Highlights tab reads "Highlights are available on macOS for now.", and clicking a highlight in the paper does nothing.
20. **Copy.** On macOS the empty Highlights panel reads "Select text in the paper, then right-click and choose Highlight." instead of the text that asks for a copy.

Unchanged: the notes editor in the panel, Insert as Quote with its review dialog and page correction, page labels and the page offset, Open in Default App, and the highlight list in search.

## Decisions

### The viewer

- **PDF.js in the existing web view** (chosen by the user). A spike ran PDF.js 6.5.438 in a MyGo web view against three arXiv papers of 15, 39 and 471 pages at 2x pixel density: 78 to 87 ms from opening to the first page drawn, 31 to 136 ms to jump to an unrendered page with its text layer, sharp text and math including older TeX fonts, and the page reported the selection with its page and position.
- **Rejected: a native `PDFView`.** MyGo has no public way to place a native view in its layout, so it needs an upstream change or a hack, and it would cover macOS only.
- **Rejected: a renderer drawn in MyGo.** Text layout, selection and scrolling from scratch is by far the most work.
- **The legacy build of PDF.js**, because the app runs on macOS 13 and the modern build targets newer WebKit.
- **PDF.js is vendored into the repository and embedded in the binary** with `go:embed`, as `internal/app/theme` embeds its icons. The app stays one file with no download at run time. The minified library, worker and viewer with the character maps, standard fonts and decoders add about 6.2 MB.
- **The viewer ships to Linux and Windows unverified** (accepted by the user). PDF.js was tried only in macOS's WebKit; WebKitGTK and WebView2 were not. The same was accepted for font discovery.
- **A scanned paper shows and offers nothing to select.** Checked in the spike with a PDF whose page is only an image: PDF.js drew it and its text layer was empty.
- **The scope of this slice is accepted by the user**: the viewer, exact highlights on macOS, navigation both ways, and zoom, links and reopening on outside changes, which were added while the spec was written. Whether today's viewer zooms by pinch is unconfirmed, since MyGo turns the web view's pinch magnification off by default; zoom may be new rather than kept.
- **Zoom is done by the page, which changes PDF.js's scale.** MyGo's `PageOptions.PinchZoom` magnifies without drawing again, which would blur the pages; it stays off. No zoom items are added to the menu bar, so the keys work only while the viewer has the keyboard.

### Highlights

- **The PDF stays the only store of highlights.** Plain files come first: a highlight is a standard annotation another reader shows too. Nothing about highlights goes into `.thesio/`.
- **PDF.js draws the highlights; the app draws no overlay of its own.** Checked in the spike: an annotation written the way `internal/pdfkit` writes one (bounds, quadrilateral points, colour, contents) is drawn by PDF.js and exposed as a clickable element with the same rectangle PDFKit wrote, to within 0.001 points. A second overlay would paint each highlight twice.
- **The page reports the selection as spans, one per line; PDFKit decides which text each is.** A span is two points in page points: where the line's selected text starts and where it ends, half a pixel inside. PDFKit selects the text between the two points and the lines of a page make one highlight, with one quadrilateral per line. The page finds lines by grouping the selected text's rectangles: a rectangle joins the line before it when the two overlap vertically and are closer sideways than one and a half line heights, so a column's line does not join the line beside it.
- **Rejected: one pair of points per page, the selection's first and last.** This was the first design and was built. It matched the browser on single-column, two-column, rotated and cropped test pages, but failed in the app on the first page of an arXiv paper: the rotated stamp in its margin comes last in the page's stored text, so a selection running off the end of the page ended in the margin, and PDFKit, reading the page in its own order, highlighted nearly the whole page. With a pair of points per line only a line's place matters.
- **Rejected: sending the browser's rectangles and writing them as the highlight.** Tried in the spike on the same three papers. The browser gives tall rectangles around inline math; the text PDFKit later reads under them took in words from the line above, and came back out of order. Between two points, PDFKit's own selection matched the browser's selected text on all three, apart from spacing inside formulas.
- **A highlight's text is what PDFKit reads under its quadrilaterals**, as today, not the text the browser selected. Highlights from other programs have no browser selection, and the quote cleanup in `internal/library/quote.go` is written for PDFKit's text. Known limit, as today: under stacked math such as a fraction, the text read back can repeat a few characters.
- **A highlight is identified by its page and bounds**, with each edge compared to within half a point. PDF.js names annotations by object reference and PDFKit by position among a page's annotations; the bounds are what both know. The selected card is remembered by page and bounds too, because the position shifts when an earlier annotation is removed. Two highlights with the same page and bounds are told apart by neither; the app writes no such pair (behavior 4), and for a pair from another program the first is taken.
- **A span with no text between its points adds nothing**, as a line of a margin note PDFKit reads differently. The write fails with `ErrNothingToHighlight` only when no span of the selection has text.
- **Several pages are written with one save.** `internal/pdfkit` rewrites the whole file on every save; one save keeps "the paper is unchanged" true when a write fails.
- **PDF.js's own comment popups are turned off.** Comments are shown and edited on the card; clicking a highlight leads there.
- **`AddHighlight` by text and `PageText` are removed** (decided by the user), with `ErrTextNotFound`, `ErrTextHasNoArea`, the `pattern` helper and the tests that use them. After this change nothing but tests would call them.
- **While the paper is opened again the viewer holds a picture of the pages in view over itself**, until those pages are drawn again. Without it the pane flashed white after every highlight.
- **The page keeps the selection when it loses the keyboard.** WebKit drops a web view's selection when the view gives up the keyboard, and a click on a button or a menu of the window does that. The page remembers the selection as it last was, draws it in grey in its place, and sends it when asked; a click or a new selection in the paper forgets it.
- **Highlight is an item of the paper's right-click menu and of the File menu, with ⌃⌘H as in Preview, and not a toolbar button** (decided by the user, who found the button out of place in the toolbar). The paper's toolbar holds Open in Default App and, last, the button that shows and hides the panel. The right-click menu is the web view's own with Reload, Back and Forward removed. There is one highlight colour, the one `internal/pdfkit` writes today.
- **Off macOS there is no Highlight item**, rather than one that fails with an error toast, which is what the stub in `internal/pdfkit` leads to.

### Go and the page

- **The viewer page is untrusted (`PageOptions.Untrusted`) and calls no Go method.** It renders files from the internet. A page of a custom scheme is otherwise trusted with every bound method and MyGo's own window functions; an untrusted page has none of that, opens no window and navigates nowhere unless the app's handlers allow it.
- **Go to page: `Page.Eval`, called from a goroutine, never from the UI thread**, where MyGo pumps a nested event loop while it waits. Results are not used for data: an untrusted page's messages over 4 KiB are dropped, and a long selection is larger.
- **Page to Go: requests to the app's custom scheme.** The page sends what it has to say (ready, opened or failed, the selection, a highlight was clicked, a web link was clicked) as requests to the scheme, and the handler finds the window's state with `mygo.CallerPage` on the request. This needs no bound service, so nothing process-wide has to be routed to a window by hand. The handler changes state only through `host.Host.Update`.
- **The same scheme serves the viewer's files and the papers.** PDF.js loads its worker and the paper with module imports and `fetch`, which need an origin; MyGo's custom schemes behave as one. Paper requests resolve through `vault.Store.Path`, so nothing outside the vault can be read. The handler answers only requests from a viewer page the app made, and only for the paper that viewer shows. The scheme replaces `Page.LoadFile`, which was there to grant the web view read access to the folder.
- **A paper is fetched whole, in one request, and never cached.** Range requests are off in PDF.js and the handler answers with `Cache-Control: no-store`; each open carries the file's stamp in its address. A file replaced between two ranges would otherwise be read half old, half new, and a cached answer would hide a new highlight. The papers are a few megabytes.
- **The viewer says when it is ready, and Go keeps only the latest thing it wants shown.** Opening a paper creates the web view and asks for the paper in one step, before the page has loaded. Go remembers the wanted paper, stamp and place; when the page reports ready, or a reopen finishes, Go sends what is wanted now. A paper replaced by another while the first is still loading is never shown.
- **The viewer reopens when the stamp of the file differs from the stamp of what it shows.** The stamp is the modification time and size, as the highlight list uses. It is compared after each write the app makes and when the vault's watcher reports the open paper changed. The app's own write therefore reopens once: the watcher's event finds the stamps equal. The highlight list's stamp is a different thing (the last list read) and is not used for this.
- **Registration fails loudly at startup.** If the scheme cannot be registered the app exits with a message, as it does when the vault cannot be opened: without it no paper can show.
- **External links go through MyGo's `Shell.OpenExternal`**, called from `internal/app/papers` as `mygo.Clipboard` is today. `host.Host.OpenExternally` takes a file path and stays as it is.

## Interfaces and data

- **Vault format:** no change. `vault.Format` stays.
- **`internal/pdfkit`** (names are binding, as the package contracts in `docs/ARCHITECTURE.md` are):

  ```go
  // Point and Rect are in page points, origin at the bottom left, as PDFKit's page space.
  type Point struct{ X, Y float64 }
  type Rect struct{ X, Y, W, H float64 }

  // Span is a run of selected text on one page, from where it starts to where
  // it ends: a viewer sends one for each line of a selection.
  type Span struct {
      Page     int
      From, To Point
  }

  type Highlight struct {
      Page          int
      Bounds        Rect // new
      Text, Comment string
      Index         int
  }

  // Same reports whether two highlights are one: same page, each edge within half a point.
  func Same(aPage int, a Rect, bPage int, b Rect) bool

  // AddHighlights highlights the text between each span's points and saves once.
  // The spans of one page make one highlight. It returns one highlight per page:
  // the new one, or the existing one with the same bounds.
  func (d *Document) AddHighlights(spans []Span, comment string) ([]Highlight, error)
  ```

  `AddHighlight(page, text, comment)`, `PageText`, `ErrTextNotFound` and `ErrTextHasNoArea` are removed, from the stub too. `AddHighlights` returns one highlight per page, so the caller can mark the card; for a page whose highlight already exists it writes nothing and returns the existing one (behavior 4), since only this package knows a highlight's bounds before writing. Errors: `ErrNoPage` for a page the document lacks, with nothing written; `ErrNothingToHighlight` when no span has text between its points; `ErrCannotWrite` from the save. The types live in `pdfkit.go` so the stub off macOS compiles; the stub returns `ErrUnavailable`.
- **The viewer page and Go.** Coordinates crossing this boundary are PDF page points with zero-based page indices, as `internal/pdfkit` uses.
  - Go to page: show a paper (its address with the stamp, and optionally a page and rectangle to bring into view and mark); reopen keeping the scroll position; close the document; send the selection.
  - Page to Go: ready; the paper was opened; the paper could not be opened (message); the selection (a list of spans, one per line, empty when nothing is selected); a highlight was clicked (page and bounds); a link to the web was clicked (its address).
- **Custom scheme:** one scheme, registered once at startup before the first window. It serves the embedded viewer files, the paper a viewer shows, and takes the page's posts. Anything else, a path `vault.Store.Path` rejects, and a request from a page that is not a viewer get an error status.
- **`internal/app/papers`:** `State` gains a way to open a paper at a highlight, used by search; `host.Host.OpenPaper` and its fakes stay as they are. `State` gains a method the workspace calls when the watcher reports a file changed.
- **Dependencies:** no new Go module or Rust crate. PDF.js 6.5.438 (Apache-2.0) is added as vendored files; its standard fonts and character maps carry their own licences.

## Affected code

- `internal/app/papers/viewer.go`:
  - `ensureWebView` makes the web view untrusted and loads the viewer page; `show`, `reload`, `loadURL` and `pageURL` are replaced by the wanted paper, stamp and place and the calls that send them. The `#page=` address and the reload counter go.
  - `PaperView`: the fallback message shows for the could-not-be-shown state as well as for a missing web view; the Highlight Selection button depends on `pdfkit.Available()`.
  - `highlightsList` gets a scroll state so a card can be brought into view, as the search panel does with `ListState.ScrollIntoView` in `internal/app/search.go`; its empty-state text changes (behaviors 19 and 20).
  - `highlightCard`: the selected look, and the card's click going to the highlight instead of `s.show(h.Page)`.
- `internal/app/papers/highlights.go`: `highlightSelection` asks the viewer for the selection instead of the clipboard and calls `AddHighlights`; `clipboardText`, `addHighlight`, `normalized` and `ErrQuoteNotInPaper` go; `changeHighlight` ends in the stamp comparison instead of `reload`; matching a highlight by page and bounds is new.
- `internal/app/papers/state.go`: the selected highlight, the viewer's stamp and wanted place; `OpenPaper` carrying an optional highlight through `WhenSaved`; `ClosePaper` closing the document in the viewer; `PathChanged` reopening at the new path instead of clearing `shown`.
- `internal/app/papers/`, new: the viewer page with its script and the vendored PDF.js files, embedded as `internal/app/theme` embeds `icons/`; the scheme's handler; and an interface between `State` and the web view (the commands above), so tests can stand in for the page. `fakeHost.Window()` returns nil today, which leaves the tests with no web view at all.
- `internal/app/external.go`: `filesChanged` tells `papers.State` when the open paper changed.
- `internal/app/search.go`: `pickSearch` opens the paper at the chosen highlight. It calls `OpenPaper` with the paper alone today.
- `internal/app` and `main.go`: registering the scheme at startup, before the first window.
- `internal/pdfkit/pdfkit.go`, `pdfkit_darwin.go`, `pdfkit_other.go`: the types and `AddHighlights` above, `Bounds` filled in by `Highlights`, and the package comment, which says there is no view and that highlights across a hyphenated break are not found. `addHighlight(page, rects, comment)` in `pdfkit_darwin.go` already builds an annotation from line rectangles and is reused. `AddHighlight`, `PageText` and `pattern` go, and `TestHighlights` and `TestHighlightAcrossLines` in `pdfkit_test.go` are rewritten for `AddHighlights`.
- `internal/app/papers/papers_test.go`: `TestHighlightSelectionHighlightsTheCopiedText` asserts the toast "Copy some text in the paper first" and is rewritten for the selection.
- `THIRD_PARTY.md`: PDF.js and what it bundles, outside the part `scripts/licences.sh` regenerates.
- `docs/ARCHITECTURE.md`: the `internal/pdfkit` and `internal/app/papers` sections, and the first entry of Limits, which describes the copy step.
- `CHANGELOG.md`: an Unreleased entry.

## Testing

Seams confirmed by the user.

1. **`internal/pdfkit` tests**, macOS only and under the `checkptr` run: `AddHighlights` with a span inside one line and one across two lines reads back after reopening with page, bounds and text; two spans on two pages are written by one call; a span on a missing page writes nothing, including the valid span beside it. Modelled on `TestHighlights` and `TestHighlightAcrossLines` in `internal/pdfkit/pdfkit_test.go`.
2. **`internal/app/papers` tests** over `fakeHost` and a temporary vault, with a fake behind the new viewer interface: Highlight Selection with a given selection writes that highlight into the PDF, selects its card and reopens the viewer once; an empty selection gives the toast and writes nothing; the same selection twice writes once; a click reported for a page and bounds selects the matching card, and one that matches nothing selects none; a card click and a search hit send the viewer the highlight's page and rectangle; a changed stamp reopens and an unchanged one does not; a paper replaced before the viewer is ready is not shown. These run under `-race`, since the handler reaches the state from another goroutine. Modelled on the tests in `internal/app/papers/papers_test.go`.
3. **The scheme's handler**, with `net/http/httptest`: the shown paper is served with `no-store`; a path leaving the vault, another vault file, and an unknown path are refused.

The viewer page's own script has no automated test in `scripts/check.sh`. It is checked by hand against the Behavior list on real papers before release, including a two-column paper and a scanned paper.

## Out of scope

- A table of contents, page thumbnails, find in the paper, and remembering the last page per paper.
- Highlight colours, a highlight button at the selection, and notes in the margin.
- Zoom items in the menu bar.
- Quoting a selection straight into a document. Insert as Quote and its dialog stay as they are.
- Adding a paper's references to the Library from the paper.
- Showing a paper beside a document, or in a second window.
- Writing or listing highlights off macOS.
- Any change to the Typst preview, including its click-to-source: it does not use the web view.

## Open questions

1. **Does the legacy build run on macOS 13's WebKit?** The spike ran only on the development Mac's current WebKit. If it does not start, behavior 16 shows the message and Open in Default App still works, and an older PDF.js release is the fallback. To be checked on a macOS 13 machine or VM before release; the user decides how.
2. **How does selecting by dragging and pinch zoom feel?** The spike selected through script and did not zoom. To be tried by hand early in the work; the user judges.
3. **Memory over a long reading session** was not measured.
