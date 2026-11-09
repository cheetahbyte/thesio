package document

import (
	"math"
	"testing"
	"time"
)

// A chapter's preview includes its main document's other pages too. Those
// pages must not be enlarged from thumbnails, even though they are not its own.
func TestPreviewPagesCoverDisplayPixels(t *testing.T) {
	files := map[string]string{
		"main.typ":    "#set page(width: 400pt, height: 300pt, margin: 24pt)\n= Main page\nSmall readable text.\n#pagebreak()\n#include \"chapter.typ\"\n",
		"chapter.typ": "= Chapter\nChapter text.\n",
	}
	d, h, tt := opened(t, files, "chapter.typ", true)
	d.SetPath("chapter.typ", "main.typ")
	until(t, h, tt, "both preview rasters", func() bool {
		return len(d.s.pages) == 2 && d.s.pages[0].bitmap != nil && d.s.pages[1].bitmap != nil
	})
	assertPreviewPixels(t, d, 1)
	before := d.s.published
	for _, tc := range []struct {
		width   int
		density float32
	}{{1337, 1.25}, {1500, 2}, {1500, 3}, {1000, 1}} {
		tt.SetSize(tc.width, 700)
		tt.SetScale(tc.density)
		tt.Frame()
		until(t, h, tt, "resolution after resize or display-scale change", func() bool { return previewPixelsMatch(d, tc.density) })
		assertPreviewPixels(t, d, tc.density)
	}
	if d.s.published != before {
		t.Fatal("resizing or changing density recompiled the document")
	}
}

func previewPixelsMatch(d *Open, density float32) bool {
	requests := rasterRequests(d.s.pages, d.pv.width, d.pv.viewH, d.pv.scroll.Y, density)
	if len(requests) == 0 {
		return false
	}
	for _, r := range requests {
		if d.s.pages[r.index].bitmap == nil || d.s.pages[r.index].key != r.key {
			return false
		}
	}
	return true
}

func assertPreviewPixels(t *testing.T, d *Open, density float32) {
	t.Helper()
	positions := layout(d.s.pages, d.pv.width)
	for i, p := range d.s.pages {
		if p.bitmap == nil {
			continue
		}
		w, height := p.bitmap.Size()
		wantW, wantH := int(math.Ceil(float64(positions[i].width*density))), int(math.Ceil(float64(positions[i].height*density)))
		if w < wantW || height < wantH {
			t.Errorf("page %d (own=%v): raster %dx%d is enlarged to %dx%d display pixels", i, p.own, w, height, wantW, wantH)
		}
	}
}

// takeUpdates holds completed callbacks apart from any later worker's updates.
// Pumping the host after another frame could also publish that frame's valid raster.
func takeUpdates(t *testing.T, h *fakeHost) []func() {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		h.mu.Lock()
		queue := h.queue
		h.queue = nil
		h.mu.Unlock()
		if len(queue) > 0 {
			return queue
		}
	}
	t.Fatal("raster worker did not queue an update")
	return nil
}

func TestPreviewRejectsObsoleteRasterAndReusesUnchangedPage(t *testing.T) {
	d, h, tt := opened(t, map[string]string{"doc.typ": "#set page(width: 400pt, height: 300pt)\nFirst page.\n#pagebreak()\nSecond page.\n"}, "doc.typ", true)
	until(t, h, tt, "initial rasters", func() bool { return len(d.s.pages) == 2 && previewPixelsMatch(d, 1) })
	original := d.s.pages[0].bitmap
	tt.SetSize(800, 700)
	tt.Frame()
	obsolete := takeUpdates(t, h) // Hold the completed, older-size render.
	tt.SetSize(1300, 700)
	tt.Frame()
	// Force the newest render to be queued too, matching the failing schedule.
	newest := takeUpdates(t, h)
	for _, update := range obsolete {
		update()
	}
	if d.s.pages[0].bitmap != original {
		t.Fatal("obsolete low-resolution result replaced the current bitmap")
	}
	for _, update := range newest {
		update()
	}
	until(t, h, tt, "newest raster", func() bool { return previewPixelsMatch(d, 1) })
	first := d.s.pages[0].bitmap
	second := d.s.pages[1].bitmap
	published := d.s.published
	d.SetText("#set page(width: 400pt, height: 300pt)\nFirst page.\n#pagebreak()\nChanged second page.\n", false)
	until(t, h, tt, "edited document's raster", func() bool { return d.s.published > published && previewPixelsMatch(d, 1) })
	if d.s.pages[0].bitmap != first {
		t.Fatal("unchanged page was rerendered")
	}
	if d.s.pages[1].bitmap == second {
		t.Fatal("changed content reused a stale bitmap")
	}
	tt.SetScale(2)
	tt.Frame()
	pending := takeUpdates(t, h)
	d.Close()
	for _, update := range pending { // A callback queued before closing must not publish.
		update()
	}
	h.pump()
	if d.s.pages[0].bitmap != first {
		t.Fatal("closed editor accepted a queued raster")
	}
}

func TestPreviewRasterBudgetAndScrollEviction(t *testing.T) {
	pages := make([]page, 200)
	for i := range pages {
		pages[i] = page{hash: "page", width: 600, height: 800}
	}
	for _, density := range []float32{1, 2, 8} {
		requests := rasterRequests(pages, 3000, 2000, 0, density)
		if len(requests) == 0 || len(requests) >= len(pages) {
			t.Fatal("not limited to the viewport")
		}
		pixels := 0
		for _, r := range requests {
			w, h := rasterSize(pages[r.index], r.scale)
			if w > rasterEdge || h > rasterEdge || w*h > pagePixels {
				t.Fatalf("unbounded page raster %dx%d", w, h)
			}
			pixels += w * h
		}
		if pixels > rasterPixels {
			t.Fatalf("retained pixels %d exceed budget", pixels)
		}
	}
	// Even an extreme, tall HiDPI viewport must render every visible page,
	// sharing the cap rather than permanently leaving the last page blank.
	requests := rasterRequests(pages, 3000, 12000, 0, 8)
	visible := 0
	for _, pos := range layout(pages, 3000) {
		if pos.top <= 12000 {
			visible++
		}
	}
	if len(requests) < visible {
		t.Fatalf("only %d of %d visible pages planned", len(requests), visible)
	}
	d, h, tt := opened(t, map[string]string{"doc.typ": "#set page(width: 400pt, height: 500pt)\n" + "One.\n#pagebreak()\nTwo.\n#pagebreak()\nThree.\n#pagebreak()\nFour.\n#pagebreak()\nFive."}, "doc.typ", true)
	until(t, h, tt, "first viewport", func() bool { return len(d.s.pages) == 5 && previewPixelsMatch(d, 1) })
	if d.s.pages[4].bitmap != nil {
		t.Fatal("offscreen last page was rasterized")
	}
	positions := layout(d.s.pages, d.pv.width)
	d.pv.scroll.Y = positions[4].top
	tt.Frame()
	until(t, h, tt, "scrolled viewport", func() bool { return d.s.pages[4].bitmap != nil && d.s.pages[0].bitmap == nil })
	if len(d.s.pages) != 5 || d.s.pages[0].width != 400 {
		t.Fatal("eviction changed page geometry or indices")
	}
}

func TestPreviewIgnoresUnmeasuredOrInvalidTargets(t *testing.T) {
	pages := []page{{width: 400, height: 300}}
	for _, values := range [][3]float32{{0, 700, 1}, {500, 0, 1}, {500, 700, 0}, {500, 700, float32(math.NaN())}, {500, 700, float32(math.Inf(1))}} {
		if requests := rasterRequests(pages, values[0], values[1], 0, values[2]); len(requests) != 0 {
			t.Fatal("invalid target scheduled raster work")
		}
	}
}
