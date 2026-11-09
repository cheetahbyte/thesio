package document

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/cheetahbyte/thesio/internal/typst"
	"github.com/egoist/mygo/ui"
)

const (
	// Bound retained RGBA pixels to 32 MiB, excluding in-flight renders and
	// the UI renderer's textures. A single page uses at most 16 MiB.
	rasterPixels = 8 * 1024 * 1024
	pagePixels   = 4 * 1024 * 1024
	rasterEdge   = 4096
)

type rasterRequest struct {
	index int
	scale float32
	key   string
}

func positiveFinite(n float32) bool {
	return n > 0 && !math.IsInf(float64(n), 0) && !math.IsNaN(float64(n))
}

func rasterSize(p page, scale float32) (int, int) {
	return max(1, int(math.Ceil(float64(p.width*scale)))), max(1, int(math.Ceil(float64(p.height*scale))))
}

// rasterScale matches the physical display width, not a fixed DPI. The caps
// protect unusually large pages and displays; only those can be undersampled.
func rasterScale(p page, width, height, density float32, budget int) float32 {
	// Typst rounds raster dimensions to the nearest pixel. Cover both axes,
	// including the partial pixel at a fractional fit-to-width zoom.
	scale := max(float32(math.Ceil(float64(width*density)))/p.width,
		float32(math.Ceil(float64(height*density)))/p.height)
	scale = min(scale, rasterEdge/p.width, rasterEdge/p.height,
		float32(budget)/p.width, float32(budget)/p.height,
		float32(math.Sqrt(float64(budget)/(float64(p.width)*float64(p.height)))))
	for {
		w, h := rasterSize(p, scale)
		if w <= rasterEdge && h <= rasterEdge && w*h <= budget {
			return scale
		}
		scale *= .999
	}
}

// rasterRequests prioritizes visible pages, then one viewport of overscan.
// There is no document-wide high-resolution bitmap cache.
func rasterRequests(pages []page, width, height, offset, density float32) []rasterRequest {
	if !positiveFinite(width) || !positiveFinite(height) || !positiveFinite(density) {
		return nil
	}
	positions := layout(pages, width)
	out := make([]rasterRequest, 0)
	remaining := rasterPixels
	visibleLeft := 0
	for i, p := range pages {
		pos := positions[i]
		if positiveFinite(p.width) && positiveFinite(p.height) && pos.top+pos.height >= offset && pos.top <= offset+height {
			visibleLeft++
		}
	}
	for pass := range 2 {
		for i, p := range pages {
			pos := positions[i]
			if !positiveFinite(p.width) || !positiveFinite(p.height) {
				continue
			}
			visible := pos.top+pos.height >= offset && pos.top <= offset+height
			near := pos.top+pos.height >= offset-height && pos.top <= offset+2*height
			if (pass == 0 && !visible) || (pass == 1 && (visible || !near)) {
				continue
			}
			budget := pagePixels
			if pass == 0 {
				// Share the budget when even the visible pages cannot all
				// fit at full density. Never leave a visible page unrendered.
				budget = min(budget, max(1, remaining/visibleLeft))
				visibleLeft--
			}
			scale := rasterScale(p, pos.width, pos.height, density, budget)
			w, h := rasterSize(p, scale)
			if w*h > remaining {
				continue
			}
			remaining -= w * h
			out = append(out, rasterRequest{index: i, scale: scale,
				key: fmt.Sprintf("%s@%g", p.hash, scale)})
		}
	}
	return out
}

// renderPreview runs on the UI thread from the preview's paint callback,
// where MyGo exposes the actual pixel scale (including content zoom). All
// expensive rendering stays on the session worker, debounced across resizes.
func (d *Open) renderPreview(density float32) {
	s := d.s
	k := s.document
	if d.closed || k == nil {
		return
	}
	requests := rasterRequests(s.pages, d.pv.width, d.pv.viewH, d.pv.scroll.Y, density)
	if len(requests) == 0 {
		return
	}
	var signature strings.Builder
	fmt.Fprintf(&signature, "%p", k)
	for _, r := range requests {
		fmt.Fprintf(&signature, "|%d:%s", r.index, r.key)
	}
	plan := signature.String()
	if plan == s.rasterPlan {
		return
	}
	s.rasterPlan = plan
	gen := s.rasterLatest.Add(1)
	known := make(map[string]*ui.Bitmap, len(s.pages))
	for _, p := range s.pages {
		if p.bitmap != nil {
			known[p.key] = p.bitmap
		}
	}
	go func() {
		time.Sleep(debounce)
		if s.rasterLatest.Load() != gen {
			return
		}
		s.work.Lock()
		defer s.work.Unlock()
		bitmaps := make(map[int]*ui.Bitmap, len(requests))
		var failed []typst.Diagnostic
		for _, r := range requests {
			if s.rasterLatest.Load() != gen {
				return
			}
			bitmap := known[r.key]
			if bitmap == nil {
				img, err := k.Render(r.index, r.scale)
				if err != nil {
					failed = append(failed, typst.Diagnostic{Error: true, Message: fmt.Sprintf("Could not render page %d: %v", r.index+1, err), Start: -1, End: -1, Line: -1})
					continue
				}
				bitmap = ui.NewBitmap(img)
				known[r.key] = bitmap
			}
			bitmaps[r.index] = bitmap
		}
		s.h.Update(func() {
			if d.closed || s.rasterLatest.Load() != gen || s.document != k {
				return
			}
			// Eviction keeps geometry and click/source indices intact. Offscreen
			// pages are white placeholders until they enter the overscan region.
			for i := range s.pages {
				s.pages[i].bitmap = nil
				s.pages[i].key = ""
			}
			for _, r := range requests {
				s.pages[r.index].bitmap = bitmaps[r.index]
				s.pages[r.index].key = r.key
			}
			s.rasterPublished = gen
			s.diagnostics = append(s.diagnostics, failed...)
		})
	}()
}
