# Paper reader implementation plan

> **Abandoned.** The tasks were implemented and then removed at the user's decision; see the Outcome in `docs/designs/paper-reader.md`. Kept as a record.
>
> Implement one task at a time. Review each deliverable before starting the next. Steps use `- [ ]`.

**Goal:** Show papers with PDF.js inside the app, so Highlight Selection highlights the selected text in place and the paper and the highlight cards lead to each other.
**Architecture:** The Paper view's web view loads a viewer page from an app-owned custom scheme, which also serves the open paper and takes the page's events. `papers.State` keeps what it wants shown and sends it to the page through a small `reader` interface; the page is untrusted and calls no Go method. Highlights stay PDF annotations written by `internal/pdfkit`, which selects the text between two points the page reports.
**Source:** `docs/designs/paper-reader.md`. Read its Behavior list first; the tasks refer to it as B1 to B20.

## Decisions

From the spec, the ones tasks depend on:

- PDF.js 6.5.438, legacy build, vendored and embedded. No download at run time.
- The PDF is the only store of highlights. PDF.js draws them; the app draws no overlay.
- The page reports a selection as one `Span` per page: the middle of the left edge of its first rectangle on that page, and the middle of the right edge of its last. PDFKit selects the text between the points and writes one quadrilateral per line.
- A highlight is identified by page and bounds, each edge within 0.5 points.
- A selection over several pages is written with one save.
- The viewer page is created with `PageOptions.Untrusted`. Go calls it with `Page.Eval` from a goroutine, never from the UI thread, and never reads an `Eval` result for data.
- A paper is fetched whole, never cached, with range requests off.
- `AddHighlight` by text, `PageText`, `ErrTextNotFound`, `ErrTextHasNoArea` and `pattern` are removed (decided by the user).
- The viewer ships to Linux and Windows unverified (accepted by the user).

Made while planning, each narrower than the spec allows:

- **The scheme is `thesio-reader`, and the paper's address carries no vault path:** `thesio-reader://localhost/paper?v=<stamp>` serves the paper the calling viewer shows. The spec asked that the handler serve "only the paper that viewer shows"; with no path in the address there is nothing to traverse. The path still comes from `vault.Store.Path`.
- **The page reports events with `GET /event?d=<JSON>`, not `POST`.** Whether WKWebView hands a `POST` body to a custom scheme was not checked; every event is small now that a selection is two points per page.
- **`AddHighlights` also answers B4.** For a span whose highlight would have the bounds of an existing highlight on that page, it writes nothing and returns the existing one. Only `internal/pdfkit` knows the bounds before writing.
- **The scheme is registered by `app.RegisterReader()`, called in `main.go` right after `app.Wire()`.** The handler reaches the vault through the `State` of the calling page, so it does not need the store at registration.
- **A link to the web is reported as an event** and opened by Go after checking its scheme, so the page needs no navigation rights.
- **The selected card is stored as page and bounds and resolved while drawing.** A click from the paper sets it at once; when the highlight list is next read and nothing matches, it is cleared. That covers B8's "answered once the list is read" without a queue.
- **A card comes into view with `ui.Element.ScrollIntoView`**, called in the one frame after the selection changes (`ui/api_gen.go:7249` in MyGo: calling it every frame would stop the user scrolling away).

## Constraints

- `CLAUDE.md` binds every task: errors handled or returned and wrapped with context when from another package; `errors.Is`, no string matching; two-value type assertions; exhaustive `switch` over enum types; no `panic` for input, files or the network; state changed from goroutines only through `host.Host.Update`; shared state guarded by a mutex; `//nolint` rare and with a reason; no `unsafe` outside `internal/typst` and `internal/pdfkit`.
- `internal/app/papers` reaches the workspace only through `host.Host`. `internal/pdfkit` imports no MyGo.
- Comments: none by default; WHY only, three lines at most, in the surrounding style.
- No new Go module and no Rust change.
- Copy, exactly:
  - toast, nothing selected: `Select some text in the paper first`
  - toast, failed write: `Could not save the highlight: ` followed by the error
  - empty Highlights panel on macOS, title `No highlights yet`, text `Select text in the paper, then press the highlighter.`
  - empty Highlights panel off macOS, title `No highlights yet`, text `Highlights are available on macOS for now.`
  - pane message, unchanged: `This PDF could not be shown here. Try opening it in your default app.`
- Never run against `~/Documents/Thesio`. Manual runs use `THESIO_HOME` set to a temporary directory.
- Task check: `CGO_ENABLED=1 go test -count=1 -race ./internal/pdfkit/ ./internal/app/...`. Before the last checkpoint: `scripts/check.sh`.
- Commit only if the user has approved commits.

## Out of scope

- A table of contents, thumbnails, find in the paper, remembering the last page.
- Highlight colours, a popover at the selection, margin notes, zoom items in the menu bar.
- Changing Insert as Quote or its dialog.
- Writing or listing highlights off macOS.
- Any change to `internal/app/document` or the Typst preview.
- Changing `host.Host` or its fakes in `document_test.go` and `citations_test.go`.

## Review focus

1. **A click or a selection event arrives for a paper that is no longer open.** Expected: ignored. Test in Task 3: an event carrying an old stamp changes nothing.
2. **The same selection highlighted twice quickly, before the first write finished.** Expected: one highlight. Test in Task 4: a second Highlight Selection while a write is under way does nothing.
3. **A reopen fails because the file is half written.** Expected: the viewer keeps the old document, no error pane, and the next change event tries again. Test in Task 3.
4. **A span whose points lie outside the page or on no text.** Expected: `ErrNothingToHighlight`, file untouched. Test in Task 1.
5. **A request to the scheme from a page that is not a registered viewer.** Expected: 403 for `/paper` and `/event`. Test in Task 2.

## Files

- `internal/pdfkit/pdfkit.go`: `Point`, `Rect`, `Span`, `Highlight.Bounds`, errors, package comment.
- `internal/pdfkit/pdfkit_darwin.go`: `AddHighlights`, bounds in `Highlights`; later the removals.
- `internal/pdfkit/pdfkit_other.go`: the stub of the same.
- `internal/pdfkit/pdfkit_test.go`: tests for `AddHighlights`.
- `internal/app/papers/reader/`, new: `index.html`, `reader.js`, `reader.css`, and `pdfjs/` (vendored).
- `scripts/vendor-pdfjs.sh`, new: reproduces `internal/app/papers/reader/pdfjs/` from the npm package.
- `internal/app/papers/scheme.go`, new: the embedded files, the scheme's handler, the registry of viewer pages, `Register`.
- `internal/app/papers/reader.go`, new: the `reader` interface, its web view implementation, the wanted and shown state, events.
- `internal/app/papers/viewer.go`: the Paper view over the reader; the panel's list and cards.
- `internal/app/papers/highlights.go`: Highlight Selection from spans, matching, the selected card.
- `internal/app/papers/state.go`: fields, `OpenPaperAt`, `FileChanged`, `ClosePaper`, `PathChanged`.
- `internal/app/papers/papers_test.go`, `scheme_test.go` (new), `reader_test.go` (new): tests.
- `internal/app/workspace.go`: `RegisterReader`.
- `internal/app/external.go`, `internal/app/search.go`, `main.go`: one call each.
- `THIRD_PARTY.md`, `docs/ARCHITECTURE.md`, `CHANGELOG.md`: documentation.

### Task 1: Highlights from spans in `internal/pdfkit`

**Files:** modify `internal/pdfkit/pdfkit.go`, `pdfkit_darwin.go`, `pdfkit_other.go`; test `internal/pdfkit/pdfkit_test.go`.
**Interfaces:**
- Consumes: `addHighlight(page objc.ID, rects []rect, comment string)` (`pdfkit_darwin.go:346`), `(*Document).run`, `(*Document).save`, `quote(page, a objc.ID) string`.
- Produces, in `pdfkit.go`:

  ```go
  // Point and Rect are in page points, origin at the bottom left, as PDFKit's page space.
  type Point struct{ X, Y float64 }
  type Rect struct{ X, Y, W, H float64 }

  // Span is a selection on one page, from where it starts to where it ends.
  type Span struct {
      Page     int
      From, To Point
  }

  type Highlight struct {
      Page          int
      Bounds        Rect
      Text, Comment string
      Index         int
  }

  // Same reports whether two highlights are one: same page, each edge within half a point.
  func Same(aPage int, a Rect, bPage int, b Rect) bool

  // AddHighlights highlights the text between each span's points and saves once.
  // It returns one highlight per span, in order: the new one, or the existing one
  // with the same bounds. Nothing is written when any span fails, or when every
  // span already has its highlight.
  func (d *Document) AddHighlights(spans []Span, comment string) ([]Highlight, error)
  ```

`AddHighlight`, `PageText` and their errors stay until Task 4, so `internal/app/papers` keeps compiling.

- [x] Rewrite `TestHighlights` and `TestHighlightAcrossLines` for `AddHighlights`, keeping their comment and remove checks, and add the tests below. `writePDF` puts "Hello highlight world" at (72, 720) and "second line here" at (72, 696) in 18-point Helvetica; a point six units above a baseline is inside its line.
  - `TestHighlights`: `AddHighlights([]Span{{Page: 0, From: Point{73, 726}, To: Point{150, 726}}}, "")` returns one highlight; after reopening, `Highlights()` has one with `Page == 0`, `strings.Contains(Text, "Hello")`, `Bounds.W > 0 && Bounds.H > 0`, and `Same` with the returned one.
  - `TestHighlightAcrossLines`: `From: Point{180, 726}, To: Point{120, 702}` gives one highlight whose text contains `world` and `second`.
  - `TestHighlightsAreWrittenTogether`: spans `{Page: 0, …valid…}` and `{Page: 1, …}` return an error for which `errors.Is(err, ErrNoPage)`, and a reopened document has no highlight.
  - `TestTheSameSpanTwiceWritesOnce`: the same valid span twice in two calls; the second returns a highlight `Same` as the first, the file's modification time is unchanged, and a reopened document has one highlight.
  - `TestASpanOnNoTextIsRefused`: `From: Point{400, 100}, To: Point{500, 100}` and a span with `math.NaN()` each give `errors.Is(err, ErrNothingToHighlight)` and no highlight; an empty slice too.
- [x] Run `CGO_ENABLED=1 go test -count=1 ./internal/pdfkit/`. Expected: does not compile, `d.AddHighlights undefined`.
- [x] Add the types and `Same` to `pdfkit.go`. Implement `AddHighlights` in `pdfkit_darwin.go`: validate every span first (page exists, four finite numbers); select with the `selectionFromPoint:toPoint:` selector on the page; take `selectionsByLine` and `boundsForPage:` as `AddHighlight` does; a selection with no line rectangle is `ErrNothingToHighlight`; compare the union of the line rectangles with the bounds of the page's existing highlight annotations using `Same`; add annotations only after every span passed, and save only if one was added. Fill `Bounds` in `Highlights` from the annotation's `bounds`. Add the stub returning `ErrUnavailable` to `pdfkit_other.go`.
- [x] Run `CGO_ENABLED=1 go test -count=1 -race ./internal/pdfkit/ && CGO_ENABLED=1 go test -count=1 -gcflags=all=-d=checkptr=2 ./internal/pdfkit/`. Expected: passes.
- [ ] Checkpoint: stage `internal/pdfkit/`. Message: `feat(pdfkit): highlight the text between two points`.

### Task 2: The vendored viewer files and the scheme

**Files:** create `scripts/vendor-pdfjs.sh`, `internal/app/papers/reader/` (with `pdfjs/`, and placeholder `index.html`, `reader.js`, `reader.css` filled in Task 3), `internal/app/papers/scheme.go`, `internal/app/papers/scheme_test.go`; modify `internal/app/workspace.go`, `main.go:28`, `THIRD_PARTY.md`.
**Interfaces:**
- Consumes: `mygo.Protocol.Handle(scheme string, handler http.Handler) error`, `mygo.FileServer(fsys fs.FS) http.Handler`, `mygo.CallerPage(ctx context.Context) *mygo.Page`, `vault.Store.Path`.
- Produces, in `scheme.go`:

  ```go
  const readerScheme = "thesio-reader"
  const readerPage = readerScheme + "://localhost/index.html"

  // Register serves the viewer page, the open paper and the page's events. Call it before the first window.
  func Register() error

  // newHandler serves the scheme; find names the State whose viewer page made a request.
  func newHandler(find func(*http.Request) (*State, bool)) http.Handler

  // register and unregister tie a viewer's page to its State, for find.
  func register(p *mygo.Page, s *State)
  func unregister(p *mygo.Page)

  // serving is the absolute path of the paper the viewer may fetch, "" for none; safe from any goroutine.
  func (s *State) serving() string
  func (s *State) setServing(path string)
  ```

  and `func RegisterReader() error` in `internal/app/workspace.go`, which returns `papers.Register()`.
- Routes: `GET /paper` answers the file at `serving()` with `Content-Type: application/pdf` and `Cache-Control: no-store`, in one piece (no `Accept-Ranges`, `Range` ignored); 403 when `find` fails, 404 when `serving()` is empty or the file is gone. `GET /event` is 403 when `find` fails and otherwise 204; Task 3 gives it its work. Every other path is the embedded `reader/` tree through `mygo.FileServer`; a path not in it is 404.

- [x] Write `scripts/vendor-pdfjs.sh`: in a temporary directory, `npm pack pdfjs-dist@6.5.438`, unpack, and copy into `internal/app/papers/reader/pdfjs/` exactly: `legacy/build/pdf.min.mjs`, `legacy/build/pdf.worker.min.mjs`, `legacy/web/pdf_viewer.mjs`, `legacy/web/pdf_viewer.css`, `legacy/web/images/`, `cmaps/`, `standard_fonts/`, `wasm/`, `iccs/`, `LICENSE`. Run it. Expected: about 6 MB under `pdfjs/`.
- [x] Write the failing tests in `scheme_test.go` with `httptest.NewRecorder`, a `State` over `fakeHost` from `vaultWithPaper`, and a `find` the test controls:
  - `TestTheViewerGetsItsPaper`: with `setServing` at the paper's absolute path, `GET /paper?v=1` is 200, its body equals the file, `Cache-Control` is `no-store`, and a `Range: bytes=0-9` request still returns the whole file with 200.
  - `TestOnlyAViewerIsAnswered`: with `find` returning false, `/paper` and `/event?d={}` are 403.
  - `TestNoPaperNoAnswer`: with `serving()` empty, `/paper` is 404.
  - `TestTheViewerFilesAreServed`: `/pdfjs/build/pdf.min.mjs` is 200 and non-empty; `/../scheme.go` and `/nothing` are 404.
- [x] Run `CGO_ENABLED=1 go test -count=1 -race ./internal/app/papers/ -run 'Viewer|Paper'`. Expected: does not compile, `undefined: newHandler`.
- [x] Implement `scheme.go`: `//go:embed reader` as in `internal/app/theme/theme.go:14`; the registry is a package map guarded by a mutex; `Register` wires `newHandler` with a `find` that looks `mygo.CallerPage(r.Context())` up in it; `serving` is a field of `State` under its own mutex. Add `RegisterReader` and call it in `main.go` after `app.Wire()`: a failure ends the app with `log.Fatalf("Could not start the paper viewer: %v", err)`, as the vault failure at `main.go:31` does.
- [x] Add a "PDF.js" section to `THIRD_PARTY.md` below `## Fonts`, outside what `scripts/licences.sh` regenerates: version 6.5.438, Apache-2.0, where it is vendored, and that its `standard_fonts/` and `cmaps/` carry the licence files beside them.
- [x] Run the task check. Expected: passes.
- [ ] Checkpoint: stage `scripts/vendor-pdfjs.sh`, `internal/app/papers/reader/`, `internal/app/papers/scheme.go`, `scheme_test.go`, `internal/app/workspace.go`, `main.go`, `THIRD_PARTY.md`. Message: `feat(papers): serve the PDF.js viewer and the open paper`.

### Task 3: The viewer page and the state that drives it

**Files:** create `internal/app/papers/reader.go`, `reader_test.go`; write `internal/app/papers/reader/index.html`, `reader.js`, `reader.css`; modify `internal/app/papers/viewer.go:15-83`, `:92-128`, `state.go`, `scheme.go` (the `/event` route), `internal/app/external.go:85`.
**Interfaces:**
- Consumes: Task 2's `readerPage`, `register`, `unregister`, `setServing`; `mygo.WebViewOptions{URL, Page: mygo.PageOptions{Untrusted: true}}`; `(*mygo.Page).Eval`, `OnRenderProcessGone(fn func(reason string))`, `Reload()`; `mygo.Shell.OpenExternal(url string) error`; `pdfkit.Rect`, `pdfkit.Span`.
- Produces, in `reader.go`:

  ```go
  // target is a place in the paper: a page and a rectangle on it, in page points.
  type target struct {
      Page   int
      Bounds pdfkit.Rect
  }

  // reader is the viewer page as State drives it. Calls return at once and reach the page in order.
  type reader interface {
      Show(version string, at *target) // open the served paper; nil is its first page
      Reopen(version string)           // open it again at the same scroll position
      GoTo(at target)                  // bring the rectangle into view and mark it briefly
      Close()                          // release the document
      RequestSelection()               // the page answers with a selection event
  }

  // event is what the page reports, decoded from /event?d=.
  type event struct {
      Type    string        `json:"type"` // ready, opened, failed, selection, clicked, link
      Version string        `json:"version"`
      Reopen  bool          `json:"reopen"`
      Message string        `json:"message"`
      Spans   []pdfkit.Span `json:"spans"`
      Page    int           `json:"page"`
      Bounds  pdfkit.Rect   `json:"bounds"`
      URL     string        `json:"url"`
  }

  func (s *State) handle(e event)    // on the UI thread
  func (s *State) sync()             // sends the reader what is wanted and not yet shown
  func (s *State) checkStamp()       // stats the open paper and reopens when its stamp changed
  func (s *State) FileChanged(id string)
  func version(st stamp) string      // "<modification time in nanoseconds>-<size>"
  ```

  `State` gains: `reader reader`; `ready bool`; `want struct{ id string; stamp stamp; at *target }`; `shown struct{ id string; stamp stamp }`; `failed bool`. `web`, `shown string` and `version int` of today go. `pdfkit.Span` and `pdfkit.Rect` get lowercase JSON tags (`page`, `from`, `to`, `x`, `y`, `w`, `h`) in `pdfkit.go`.
- The page, `reader.js`, exposes `window.reader` with `show(version, at)`, `reopen(version)`, `goTo(at)`, `close()` and `sendSelection()`, and reports with `fetch("/event?d=" + encodeURIComponent(JSON.stringify(event)))`.

Rules the code must follow, where the signatures leave a choice:

- `sync`: nothing while `!ready` or `want.id == ""`. If `failed` or `shown.id != want.id`: `setServing`, `reader.Show`, and clear `want.at`. Else if `shown.stamp != want.stamp`: `reader.Reopen`. Else if `want.at != nil`: `reader.GoTo`, and clear it.
- `handle`: `ready` sets `ready`, clears `shown` and calls `sync`. `opened` whose version is `version(want.stamp)` sets `shown` and clears `failed`; any other version is ignored. `failed` with `Reopen` changes nothing (B14); without it, and for the wanted version, sets `failed` (B16). `link` opens the URL with `mygo.Shell.OpenExternal` only when its scheme is `http`, `https` or `mailto`, and toasts a failure. `selection` and `clicked` are Task 4's; here they are ignored.
- The web view implementation of `reader` owns one goroutine that takes scripts from a channel and runs `Eval` one after another, so calls stay in order and never run on the UI thread. An `Eval` error is dropped: the ready timeout below covers a page that does not answer.
- `ensureWebView` creates the web view with `URL: readerPage` and `Untrusted: true`, calls `register`, and starts a 10-second timer that, through `host.Host.Update`, sets `failed` if `ready` is still false (B16). `OnRenderProcessGone` clears `ready` and `shown` through `Update` and calls `Reload` (B18).
- `OpenPaper` sets `want` (id, the stamp from `os.Stat`, no target), clears `failed` when the paper differs, and calls `sync` in place of `s.show(-1)`. `ClosePaper` calls `reader.Close()`, `setServing("")` and clears `want` and `shown` (B17). `PathChanged` moves `want.id` and `shown.id`, updates `serving`, and sends nothing: the document in the page is the same file (B15).
- `checkStamp` stats the open paper; when the stamp differs from `want.stamp` it sets it and calls `sync`. A failed stat does nothing. `FileChanged(id)` calls it when `id == s.current`. `filesChanged` in `internal/app/external.go` calls `ws.papers.FileChanged(e.Path)` for every event; `e.Path` is vault-relative with slashes, as its comparison with `vault.LibraryFile` there shows.
- `changeHighlight` and `highlightSelection` end in `checkStamp` instead of `reload` (the cache line `e.read = false` of `reload` moves with them).
- `PaperView` shows the pane message when `s.reader == nil || s.failed`.
- The page:
  - `index.html` loads `pdfjs/web/pdf_viewer.css`, `reader.css` and `reader.js` as a module. `reader.js` sets `globalThis.pdfjsLib` from `./pdfjs/build/pdf.min.mjs` before importing `./pdfjs/web/pdf_viewer.mjs`, sets `GlobalWorkerOptions.workerSrc` to the worker file, and builds an `EventBus`, a `PDFLinkService` and a `PDFViewer` on a scrolling container. It reports `ready` once that is done.
  - `show` and `reopen` call `getDocument` with `url: "/paper?v=" + version`, `disableRange: true`, `disableStream: true`, `disableAutoFetch: true`, and `cMapUrl`, `standardFontDataUrl`, `wasmUrl`, `iccUrl` under `./pdfjs/`. Only after the new document loaded is it given to the viewer, so a failed `reopen` leaves the old one (B14). `show` starts at fit width (`currentScaleValue = "page-width"`) and its target or the first page; `reopen` restores scale and scroll offset. Both report `opened` or `failed` with the version and whether it was a reopen.
  - `goTo` scrolls so the rectangle is in view (convert with the page view's `viewport.convertToViewportPoint`) and marks it with an element that fades after about a second.
  - `sendSelection` reports `selection` with one span per page the selection touches: for the selection's client rectangles on that page wider and taller than one pixel, in document order, `from` is the middle of the left edge of the first plus half a point, `to` the middle of the right edge of the last minus half a point, converted with `viewport.convertToPdfPoint` from coordinates relative to the page's text layer. No selection reports an empty list.
  - A click on an element with class `highlightAnnotation` in the annotation layer reports `clicked` with the page index and the annotation's rectangle, taken from `getAnnotations()` by the element's `data-annotation-id`, as x, y, width and height.
  - `reader.css` hides PDF.js's `.popupAnnotation` elements.
  - A click on a link whose address has another origin is cancelled and reported as `link`.
  - Zoom: ⌘ or Ctrl with `+`/`=`, `-` and `0` change `currentScale` by a factor of 1.1 or return to `page-width`; WebKit's `gesturechange` and a `wheel` event with `ctrlKey` do the same continuously, clamped to 0.25 to 5 (B12).

- [x] Write the failing tests in `reader_test.go` with a `fakeReader` that records calls as strings (`show <version> <page or ->`, `reopen <version>`, `goto <page>`, `close`, `selection`):
  - `TestThePaperIsSentOnceTheViewerIsReady`: `OpenPaper` records nothing; `handle(event{Type: "ready"})` records one `show` with the file's version.
  - `TestOnlyTheLatestPaperIsShown`: two `OpenPaper` calls for two papers, then `ready`: one `show`, for the second.
  - `TestAChangedFileReopens`: after `opened`, rewriting the file with another size and `FileChanged(paperID)` records `reopen`; a second `FileChanged` with the file untouched records nothing; `FileChanged("other.pdf")` records nothing.
  - `TestAFailedReopenKeepsThePaper`: `handle(event{Type: "failed", Reopen: true, Version: …})` leaves `failed` false.
  - `TestAPaperThatCannotOpenSaysSo`: `failed` without `Reopen` for the wanted version sets `failed`; for an older version it does not (Review focus 1).
  - `TestLeavingClosesTheDocument`: `ClosePaper` records `close` and `serving()` is empty.
  - `TestEventsReachTheStateThroughTheScheme`, in `scheme_test.go`: `GET /event?d=` with a `ready` event is 204 and, after `f.settle()`, the reader has a `show`; a body that is not JSON is 400.
- [x] Run the task check. Expected: does not compile, `undefined: fakeReader`'s interface and `handle`.
- [x] Implement `reader.go`, the changes to `state.go`, `viewer.go`, `scheme.go` and `external.go`, and the three page files, following the rules above.
- [x] Run the task check. Expected: passes.
- [x] Run the app on a temporary vault (fish: `set -x THESIO_HOME (mktemp -d); CGO_ENABLED=1 go run .`), add a PDF on the Sources page and open it. Expected: the paper shows fitted to the width with selectable text; pinch and ⌘+ zoom and stay sharp; an internal citation link scrolls; a web link opens the browser; replacing the file from a terminal shows the new file at the same place. Highlight Selection still uses the clipboard until Task 4.
- [ ] Checkpoint: stage `internal/app/papers/`, `internal/pdfkit/pdfkit.go`, `internal/app/external.go`. Message: `feat(papers): show papers with PDF.js`.

### Task 4: Highlights in place, and the paper and cards leading to each other

**Files:** modify `internal/app/papers/highlights.go`, `viewer.go:92-224`, `state.go`, `reader.go` (`handle`), `internal/app/search.go:192`, `internal/pdfkit/pdfkit.go`, `pdfkit_darwin.go`, `pdfkit_other.go`, `pdfkit_test.go`; test `internal/app/papers/papers_test.go`.
**Interfaces:**
- Consumes: Task 1's `AddHighlights`, `Same`, `Highlight.Bounds`; Task 3's `reader`, `target`, `event`, `checkStamp`, `want`.
- Produces:

  ```go
  // OpenPaperAt shows paper id with highlight h in view and its card selected.
  func (s *State) OpenPaperAt(id string, h pdfkit.Highlight)
  ```

  `State` gains `selected *target`, `reveal bool` (the selected card comes into view in the next frame) and `writing bool` (a highlight is being written). `clipboardText`, `addHighlight`, `normalized` and `ErrQuoteNotInPaper` in `highlights.go` go.

Rules:

- The Highlight Selection button calls `reader.RequestSelection()` unless `writing`. `handle` for `selection`: no spans gives the toast `Select some text in the paper first` (B5); otherwise set `writing` and, on a goroutine, open the document and call `AddHighlights(spans, "")`. Back through `host.Host.Update`: clear `writing`; on an error toast `Could not save the highlight: ` and the error (B6); otherwise show the panel on Highlights, set `selected` to the first returned highlight and `reveal`, mark the paper's cache entry unread, and call `checkStamp` (B2, B3, B4).
- `handle` for `clicked`: show the panel on Highlights, set `selected` to the event's page and bounds and `reveal` (B7 is the card's side, B8 this one). Off macOS the event is ignored (B19).
- When the highlight list of the open paper has been read (the `Update` callback in `indexed`), clear `selected` if no highlight is `pdfkit.Same` as it (B8, B10).
- `highlightCard`: a card whose highlight is `Same` as `selected` gets the selected look (`theme.Selected(t)` as its background, as the panel button at `viewer.go:106` uses) and, when `reveal` is set, `ScrollIntoView()` once; clear `reveal` after the list is built. A click on the card's text sets `selected` and calls `reader.GoTo(target{h.Page, h.Bounds})` in place of `s.show(h.Page)` (B7).
- `OpenPaperAt` is `OpenPaper` with `want.at` and `selected` set to the highlight, `reveal` set, and the panel on Highlights, all inside the same `WhenSaved` callback (B9). `pickSearch` in `internal/app/search.go` calls `ws.papers.OpenPaperAt(h.paper, h.highlight)` for `hitHighlight`.
- `PaperView` builds the Highlight Selection button only when `pdfkit.Available()`. `highlightsList`'s empty state uses the two texts under Constraints, chosen by `pdfkit.Available()` (B19, B20).
- Remove from `internal/pdfkit`: `AddHighlight`, `PageText`, `pattern`, `ErrTextNotFound`, `ErrTextHasNoArea`, the selectors only they use, and their stubs. Replace the package comment in `pdfkit.go`: it reads and annotates PDFs through PDFKit; a highlight is the text between two points on a page.

- [x] Write the failing tests in `papers_test.go`, each skipping when `!pdfkit.Available()`, using the `fakeReader` of Task 3 and `writePDF` there (the same two lines as in `internal/pdfkit`):
  - Replace `TestHighlightSelectionHighlightsTheCopiedText` with `TestHighlightSelectionHighlightsTheSelection`: the button's action records `selection`; `handle(event{Type: "selection"})` with no spans gives exactly the error `Select some text in the paper first`; with `Spans: []pdfkit.Span{{Page: 0, From: pdfkit.Point{X: 180, Y: 726}, To: pdfkit.Point{X: 120, Y: 702}}}` and `f.settle()`: `s.tab == tabHighlights`, no further error, `s.selected != nil`, the reader recorded one `reopen`, and after the list is read one highlight whose text contains `second`.
  - `TestTheSameSelectionTwiceWritesOnce`: the same event twice, settling between; one highlight, and `s.selected` matches it.
  - `TestHighlightSelectionWaitsForTheWriteUnderWay`: with `s.writing` set, the button's action records nothing (Review focus 2).
  - `TestAClickInThePaperSelectsItsCard`: after one highlight exists and the list is read, `handle(event{Type: "clicked", Page: 0, Bounds: <its bounds>})` sets `selected` and `tab`; a click with bounds 50 points away is cleared once the list is read again.
  - `TestACardLeadsToItsHighlight`: the card's action records `goto 0`.
  - `TestASearchHitOpensThePaperAtTheHighlight`: `OpenPaperAt(paperID, h)` then `ready` records `show <version> 0`, and `s.selected` matches `h`.
- [x] Run the task check. Expected: fails to compile on `OpenPaperAt` and `s.selected`.
- [x] Implement the rules above, then remove the old API from `internal/pdfkit` and the tests that used it.
- [x] Run the task check and `rg -n 'AddHighlight\(|PageText|ErrTextNotFound|ErrTextHasNoArea|clipboardText|ErrQuoteNotInPaper' --type go`. Expected: tests pass; the search finds nothing.
- [ ] Run the app on a temporary vault as in Task 3 and go through B2 to B11 on a real paper, including a selection across a page break and a selection around inline math. Expected: each behaves as the spec says. Then try a two-column paper and a scanned one (B5).
- [ ] Checkpoint: stage `internal/app/papers/`, `internal/app/search.go`, `internal/pdfkit/`. Message: `feat(papers): highlight the selection in place and link the paper with its cards`.

### Task 5: Documentation and the full check

**Files:** modify `docs/ARCHITECTURE.md:325-338`, `:423-448`, `:468-470`, `CHANGELOG.md`, `docs/designs/paper-reader.md`.

- [x] `docs/ARCHITECTURE.md`: in `### internal/pdfkit` replace `AddHighlight` and `PageText` with `Point`, `Rect`, `Span`, `Same`, `AddHighlights` and `Highlight.Bounds`. In `### internal/app/papers` add `OpenPaperAt` and `FileChanged`, and replace the paragraph on the WKWebView viewer with: the paper shows in a `ui.WebView` on an untrusted page of the `thesio-reader` scheme running PDF.js; Go drives it through `reader`, the page reports through `/event`; highlights are PDF annotations written from the selection's end points. In `## Limits` replace the first entry with what holds now: highlights are written on macOS only; a selection that starts inside a figure's labels can take in neighbouring labels; text read back under stacked math can repeat characters.
- [x] `CHANGELOG.md`, under `## Unreleased`, in the file's voice: "Read papers in a viewer of Thesio's own. Highlight Selection now highlights the text you selected, where you selected it, without copying it first. Click a highlight's card to go to it in the paper, click a highlight in the paper to find its card, and open a highlight from search at its place. Pinch or ⌘+ and ⌘− to zoom."
- [x] `docs/designs/paper-reader.md`: set **Status:** to `implemented`.
- [x] Run `scripts/check.sh`. Expected: every step passes, including `golangci-lint`, `govulncheck` and the `checkptr` run.
- [ ] Checkpoint: stage `docs/`, `CHANGELOG.md`. Message: `docs: describe the paper reader`.

## Deviations

Filled in during implementation. The tasks above stay as written. One entry per change:

- **needs decision** · Tasks 1, 3 and 4 · conflicts with: the Decision "The page reports a selection as one `Span` per page" and Task 1's Interfaces (one highlight per span, "Nothing is written when any span fails") · chose: the page sends one span per line of the selection, the spans of a page make one highlight, `AddHighlights` returns one highlight per page, and a span with no text adds nothing (`ErrNothingToHighlight` only when no span has text) · alternative: keep one pair of points per page · why: in the app, a selection running off the end of the first page of an arXiv paper ended in the rotated stamp in the margin, which is last in the page's stored text, and PDFKit highlighted nearly the whole page. The spec named this fallback and left the choice to the user. The spec is updated to describe what was built.
- Task 1, tests · planned: the across-lines span ends at (120, 702) · did: (140, 702), in `internal/app/papers` too · why: at 120 PDFKit cut "second" short. Added `TestTheLinesOfAPageMakeOneHighlight` for the change above.
- Task 2 · planned: the PDF.js section of `THIRD_PARTY.md` below `## Fonts` · did: a `## PDF viewer` section between Fonts and Icons, which `scripts/licences.sh` leaves alone as well.
- Task 2 · planned: `mygo.FileServer` for the embedded files, and placeholder page files · did: a small handler over `embed.FS` with `http.ServeContent`; no placeholders · why: `mygo.FileServer` answers unknown paths with `index.html` and `http.FileServer` redirects `/index.html` to `/`; the plan wants 404.
- Task 3, `State` · planned: `web` goes; `want` holds the target · did: `web` stays (the `ui.WebView` element needs it); added `asked`, what the reader was last told, and `at`, the place still to go to · why: without `asked`, a second `sync` before the page answered would send the paper twice.
- Task 3, `handle` · planned: `selection` and `clicked` carry no check · did: both are ignored unless their version is the one the page last reported open for the current paper · why: Review focus 1.
- Task 3, the page · planned: `goTo` "scrolls so the rectangle is in view" · did: it leaves the scroll alone when the highlight is already in view, otherwise puts it a third down the pane. The half-point inset of a span's points is half a pixel, applied before converting, so it holds on rotated pages.
- Task 3, the page · planned: a click on an element of class `highlightAnnotation` · did: the page tests the click's place against the page's highlight annotations itself · why: it does not depend on which elements PDF.js creates for a highlight without a comment.
- Task 3, `reader.css` · added: PDF.js's `.annotationCommentButton` and `.commentPopup` are hidden too · why: a button showed beside a new highlight. Read in PDF.js's source; not seen gone in the app, whose window was hidden at the time.
- Task 4, `selectionArrived` · added: `pdfkit.ErrNothingToHighlight` shows `Select some text in the paper first`, not the failed-write toast · why: a selection of blank space is not a failed write.
- Tasks 3 and 4, tests · added: `settleUntil`, `written`, `highlightsRead`, `TestASelectionOfAnotherStateOfTheFileIsIgnored`, `TestAMovedPaperStaysOpen`, `TestOnlyWebAndMailLinksGoOut` · why: opening a paper also loads its notes, whose updates `fakeHost.settle` mistook for the awaited one.
- Tasks 3 and 4, the runs in the app · planned: by hand · did: an `e2e` build on a temporary vault, driven through MyGo's automation socket, with a temporary hook (since removed) that ran scripts in the page to select text and click. Seen working on the 15-page test paper: opening, highlighting three lines from mid-line to mid-line, a selection over a page break, a passage through inline and displayed math, the card going to its highlight, a click in the paper selecting its card, keyboard zoom and back to fit, deleting a highlight, a half-written file kept out and the whole file shown again at the same place. Not exercised: dragging with a real mouse, pinch, a web link (it would open the browser), and a two-column and a scanned paper inside the app (both were tried in the spike only).
- Task 3, the page · added after the user's first try: the page keeps the last selection and shows it in grey while it is without the keyboard, and sends that when asked · why: WebKit drops a web view's selection when the view gives up the keyboard, which a click on the Highlight Selection button causes, so every real selection arrived empty ("Select some text in the paper first"). The scripted runs never gave the web view the keyboard and missed it. Checked by script only, with the loss of the keyboard imitated; a real drag and click is the user's to confirm.
- Task 3, the page · changed after the user's second try: only the selection's range is kept while dragging, and its lines are worked out when needed · why: working them out on every selection change made dragging slow.
- Task 3, the page · added after the user's second try: while the paper is opened again, a picture of the pages in view covers the viewer until they are drawn · why: the pane flashed white after each highlight.
- **needs decision, decided by the user** · Task 4 · conflicts with: the Decision "Highlight Selection stays a toolbar button" · did: the toolbar button is gone; Highlight is the first item of the paper's right-click menu and File > Highlight Selection, both ⌃⌘H; the toolbar is Open in Default App, then the panel button; the empty panel reads "Select text in the paper, then right-click and choose Highlight." · why: the user found the button out of place and chose the right-click menu. `State.HighlightSelection` is exported for the menu; `app.CmdHighlight` and `app.HighlightKey` are new.
- Checkpoints · not committed: the user has not approved commits.
