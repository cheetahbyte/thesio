// Package theme holds what the app's views share beyond MyGo's theme: the icon
// set (Lucide, embedded from icons/), the monospace font, and a few colors
// MyGo's theme has no name for. Everything else comes from c.Theme().
package theme

import (
	"embed"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var files embed.FS

// Icon is a glyph: an SF Symbol on macOS, the Lucide SVG elsewhere.
type Icon struct {
	symbol string
	svg    *ui.SVG
}

func icon(symbol, file string) Icon {
	data, err := files.ReadFile("icons/" + file + ".svg")
	if err != nil {
		panic(err)
	}
	return Icon{symbol, ui.MustParseSVG(data)}
}

// Glyph draws i size DIPs tall in col.
func Glyph(c *ui.Context, i Icon, size float32, col ui.Color) ui.Element {
	if sfsymbols.Available() {
		return sfsymbols.Symbol(c, i.symbol).Natural().Size(size).Color(col).Fallback(i.svg).Element
	}
	return ui.Icon(c, i.svg).FontSize(size).TextColor(col)
}

// SidebarGlyph draws i as Finder's sidebar draws its symbols: 23 DIP, and a
// medium weight, since a symbol rasterized large and scaled down keeps a
// thinner stroke than one drawn at that size.
func SidebarGlyph(c *ui.Context, i Icon, col ui.Color) ui.Element {
	const size = 23
	if sfsymbols.Available() {
		return sfsymbols.Symbol(c, i.symbol).Natural().Weight(sfsymbols.Medium).Size(size).Color(col).Fallback(i.svg).Element
	}
	return ui.Icon(c, i.svg).FontSize(size).TextColor(col)
}

// Mono names the system's monospaced font for ui.Font and editor.State.Font.
const Mono = "monospace"

// Icons are the app's glyphs, drawn with Glyph.
var Icons = struct {
	Home, HomeFilled, Library, Paper, Doc, Gear, Folder, FolderNew         Icon
	ChevronRight, ChevronDown, ChevronUp, Plus, Ellipsis, Search, Clear    Icon
	Delete, Edit, Move, Copy, Pin, Unpin, Star, Unstar, Export, Import     Icon
	Restore, Quote, Notes, Tag, Check, Minus, Close, Show, Highlight       Icon
	External, Panel, Back, Palette, Storage, Info, Select, Expand, Sidebar Icon
	Dot                                                                    Icon
}{
	Home: icon("house", "home"), HomeFilled: icon("house.fill", "home-filled"), Library: icon("books.vertical", "library"),
	Paper: icon("doc.text", "doc"), Doc: icon("doc.text", "doc"),
	Gear: icon("gearshape", "settings-gear"), Folder: icon("folder", "folder"), FolderNew: icon("folder.badge.plus", "folder-plus"),
	ChevronRight: icon("chevron.right", "chevron-right"), ChevronDown: icon("chevron.down", "chevron-down"), ChevronUp: icon("chevron.up", "chevron-up"),
	Plus: icon("plus", "plus"), Ellipsis: icon("ellipsis", "ellipsis"), Search: icon("magnifyingglass", "search"), Clear: icon("xmark.circle.fill", "search-clear"),
	Delete: icon("trash", "bin"), Edit: icon("pencil", "pen-line"), Move: icon("arrowshape.turn.up.right", "folder-input"), Copy: icon("doc.on.doc", "copy"),
	Pin: icon("pin", "pin"), Unpin: icon("pin.slash", "pin-off"), Star: icon("star", "star"), Unstar: icon("star.slash", "star-off"),
	Export: icon("square.and.arrow.up", "export"), Import: icon("square.and.arrow.down", "import"), Restore: icon("clock.arrow.circlepath", "restore"),
	Quote: icon("quote.opening", "quote"), Notes: icon("square.and.pencil", "square-pen"), Tag: icon("tag", "tag"), Check: icon("checkmark", "check"),
	Minus: icon("minus", "minus"), Close: icon("xmark", "x"), Show: icon("eye", "eye"), Highlight: icon("highlighter", "square-pen"),
	External: icon("arrow.up.forward.app", "export"), Panel: icon("sidebar.trailing", "panel-right"), Back: icon("arrow.left", "arrow-left"),
	Palette: icon("paintpalette", "palette"), Storage: icon("archivebox", "storage"), Info: icon("info.circle", "info"),
	Select: icon("checklist", "batch"), Expand: icon("arrow.up.left.and.arrow.down.right", "expand"), Sidebar: icon("sidebar.leading", "panel-left-close"),
	Dot: icon("circle.fill", "circle-filled"),
}

// BarHeight is the height of every top bar, the traffic lights centered in it.
const BarHeight = 52

// TableInset keeps a table off the pane's edges, so its cells' text lines up with the bar's title.
const TableInset = 10

// TrafficLights is where the close button's corner sits so that the lights
// center in a BarHeight bar, as in Finder's unified toolbar.
const TrafficLights = 18

// Caption is secondary text: dates, meta, captions of controls.
func Caption(t *ui.Theme) ui.Color { return t.TextMuted }

// Danger colors destructive actions and errors.
func Danger(t *ui.Theme) ui.Color { return t.Danger }

// ink is the text color at alpha: translucent, so it reads over the sidebar material too.
func ink(t *ui.Theme, light, dark float32) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, dark)
	}
	return ui.RGBA(0, 0, 0, light)
}

// Hairline is one line at the window's seams, as Finder's separators.
func Hairline(t *ui.Theme) ui.Color { return ink(t, 0.1, 0.1) }

// Hover tints a row or a toolbar button under the pointer.
func Hover(t *ui.Theme) ui.Color { return ink(t, 0.05, 0.07) }

// Selected tints the chosen row of a list that does not have the focus.
func Selected(t *ui.Theme) ui.Color { return ink(t, 0.09, 0.12) }

// Group tints grouped rows, as settings and forms, set off by tone instead of an outline.
func Group(t *ui.Theme) ui.Color { return t.Surface }

// Ring outlines an image or a raised panel at 10%, so its edge holds on any background.
func Ring(t *ui.Theme) ui.Color { return ink(t, 0.1, 0.1) }

// Elevate lifts a panel (a dialog, the search panel) with a ring and two soft layers.
func Elevate(e ui.Element, t *ui.Theme) ui.Element {
	return e.Shadow(0, 0, 0, 1, Ring(t)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.06)).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.14))
}

// Scrim dims the window behind a dialog, lightly, as macOS dims it behind a sheet.
var Scrim = ui.RGBA(0, 0, 0, 0.2)

// PanelRadius rounds dialogs and the search panel, as macOS 26 rounds its sheets.
const PanelRadius = 22

// Panel raises a dialog's or a popover's panel on glass, so that what is behind it tints it.
func Panel(panel ui.Element, t *ui.Theme) ui.Element {
	tint := t.Background
	tint.A = 204
	return Elevate(panel.Material(glass.Glass{Tint: tint}), t)
}

// Modal is ui.Modal with the look every dialog has: a light scrim and a round panel of glass.
func Modal(c *ui.Context, open *bool, fn func()) ui.Element {
	t := c.Theme()
	width, height := c.Size()
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(Scrim)
		Panel(panel.Padding(20).Gap(12).Radius(PanelRadius).MaxWidth(width-40).MaxHeight(height-40), t)
		fn()
	})
}

// Button is a push button, a capsule as macOS 26's.
func Button(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Radius(capsule).BorderWidth(0)
}

// PrimaryButton is a dialog's or a page's main action, an accent capsule.
func PrimaryButton(c *ui.Context, label string) ui.Element {
	return ui.PrimaryButton(c, label).Radius(capsule)
}

// capsule is a radius past half of any control's height.
const capsule = 100

// Table builds a table as Finder's list view shows one: 24 DIP rows under an
// 11 DIP header. The cells set their own text sizes.
func Table(c *ui.Context, build func()) ui.Element {
	t := *c.Theme()
	t.Spacing, t.FontSize = 3, 11
	return ui.Themed(c, &t).Background(ui.Transparent).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(build)
}

// Stripe tints every other row of a table, as Finder's list view does.
func Stripe(t *ui.Theme) ui.Color { return ink(t, 0.04, 0.05) }

// Rows tints every other row of a table built in Table, and the room under
// its last row, as Finder's list view: MyGo's table has no such option, so each
// cell paints its part of its row's stripe. Make one a frame.
type Rows struct {
	n     int
	table ui.Element
	last  ui.Rect
	seen  bool
}

// NewRows returns the stripes of a table of n rows.
func NewRows(n int) *Rows { return &Rows{n: n} }

// Cell builds a cell's content over its row's stripe; a chosen row is plain, its highlight showing.
func (r *Rows) Cell(c *ui.Context, row int, chosen bool, content func()) {
	t := c.Theme()
	// The cell's own padding above and under its content, which the stripe covers too.
	pad := t.Space(1.5)
	ui.Row(c).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).AlignItems(ui.Center).Draw(func(p *ui.Painter, box ui.Rect) {
		table := r.table.Bounds()
		stripe := ui.Rect{X: table.X, Y: box.Y - pad, W: table.W, H: box.H + 2*pad}
		if row == r.n-1 {
			r.last, r.seen = stripe, true
		}
		// Spanning the table and cut to the cell, the cells' parts make one stripe with round ends.
		if row%2 == 1 && !chosen {
			p.Fill(stripe, Stripe(t), stripeRadius)
		}
	}).Children(content)
}

// Under continues the stripes under the last row of table, which Cell's cells belong to.
func (r *Rows) Under(table ui.Element) {
	r.table = table
	table.DrawOver(func(p *ui.Painter, box ui.Rect) {
		if !r.seen || r.last.H <= 0 {
			return
		}
		t := p.Theme()
		bottom := box.Y + box.H
		for row, y := r.n, r.last.Y+r.last.H; y < bottom; row, y = row+1, y+r.last.H {
			if row%2 == 1 {
				p.Fill(ui.Rect{X: box.X, Y: y, W: box.W, H: min(r.last.H, bottom-y)}, Stripe(t), stripeRadius)
			}
		}
	})
}

const stripeRadius = 5

// Tabular sets digits in equal widths, for counts and dates that change or line up.
func Tabular(e ui.Element) ui.Element { return e.FontFeatures("tnum") }

// IconButton is a 28 DIP square toolbar button showing glyph, named label for
// the tooltip and assistive technology.
func IconButton(c *ui.Context, glyph Icon, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(28, 28).Shrink(0).Center().Radius(6).TextColor(t.TextMuted)
	if b.Hovered() {
		b.Background(Hover(t))
	}
	b.Children(func() { Glyph(c, glyph, 17, t.TextMuted) })
	return b
}

// BarControl is the height of a bar's controls, and BarRadius rounds them into capsules.
const (
	BarControl = 36
	BarRadius  = BarControl / 2
)

// BarGroup holds a bar's buttons side by side.
func BarGroup(c *ui.Context, buttons func()) ui.Element {
	return ui.Row(c).Height(BarControl).Shrink(0).AlignItems(ui.Center).Padding(0, 4).Children(buttons)
}

// BarButton is a button of a BarGroup showing glyph, named label for the
// tooltip and assistive technology.
func BarButton(c *ui.Context, glyph Icon, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(36, 28).Shrink(0).Center().Radius(14)
	if b.Hovered() {
		b.Background(Hover(t))
	}
	b.Children(func() { Glyph(c, glyph, 16, t.Text) })
	return b
}

// Title is a bar's title: 15 DIP semibold, one line.
func Title(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(15).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
}

// Count is the muted count beside a bar's title.
func Count(c *ui.Context, n string) ui.Element {
	return Tabular(ui.Text(c, n).FontSize(15).TextColor(c.Theme().TextMuted).SingleLine())
}

// EmptyState fills a page that has nothing to show, more under the text.
func EmptyState(c *ui.Context, glyph Icon, title, text string, more func()) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(4).Padding(24, 32, 64).Children(func() {
		Glyph(c, glyph, 32, t.TextMuted).Margin(0, 0, 8, 0)
		ui.Text(c, title).FontSize(15).FontWeight(600)
		ui.Text(c, text).FontSize(13).TextColor(Caption(t)).TextAlign(ui.Center)
		if more != nil {
			more()
		}
	})
}

// AddButton is a bar's primary action: a plus and label in an accent capsule, as macOS 26's toolbars.
func AddButton(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	return ui.PrimaryButton(c, "").Label(label).Height(BarControl).Padding(0, 16, 0, 13).Gap(5).Radius(BarRadius).Children(func() {
		Glyph(c, Icons.Plus, 12, t.AccentText)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(t.AccentText).SingleLine()
	})
}
