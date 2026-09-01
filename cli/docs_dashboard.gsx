package main

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
)

// docsDashboard is the terminal doc viewer scaffold (issue #379). It wraps
// go-tui's built-in <markdown> element in a scrollable container — same
// Ref+scrollY shape as metricsDashboard (metrics_dashboard.gsx), just with
// a single markdown block instead of the metrics panels.
//
// TODO(#379): this only shows whatever loadDoc() returns for one doc name.
// Doc switching, a picker, and search are not built yet.
type docsDashboard struct {
	source     *tui.State[docsSource]
	scrollY    *tui.State[int]
	contentRef *tui.Ref
}

func DocsDashboard(docName string) *docsDashboard {
	return &docsDashboard{
		source:     tui.NewState(loadDoc(docName)),
		scrollY:    tui.NewState(0),
		contentRef: tui.NewRef(),
	}
}

func (d *docsDashboard) scrollBy(delta int) {
	el := d.contentRef.El()
	if el == nil {
		return
	}
	_, maxY := el.MaxScroll()
	newY := d.scrollY.Get() + delta
	if newY < 0 {
		newY = 0
	} else if newY > maxY {
		newY = maxY
	}
	d.scrollY.Set(newY)
}

func (d *docsDashboard) KeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.On(tui.KeyEscape, func(ke tui.KeyEvent) { ke.App().Stop() }),
		tui.On(tui.Rune('q'), func(ke tui.KeyEvent) { ke.App().Stop() }),
		tui.On(tui.Rune('j'), func(ke tui.KeyEvent) { d.scrollBy(1) }),
		tui.On(tui.Rune('k'), func(ke tui.KeyEvent) { d.scrollBy(-1) }),
		tui.On(tui.KeyDown, func(ke tui.KeyEvent) { d.scrollBy(1) }),
		tui.On(tui.KeyUp, func(ke tui.KeyEvent) { d.scrollBy(-1) }),
	}
}

func (d *docsDashboard) HandleMouse(me tui.MouseEvent) bool {
	switch me.Button {
	case tui.MouseWheelUp:
		d.scrollBy(-1)
		return true
	case tui.MouseWheelDown:
		d.scrollBy(1)
		return true
	}
	return false
}

templ (d *docsDashboard) Render() {
	<div class="flex-col p-1 gap-1 h-full border-rounded border-cyan">
		<div class="flex justify-center shrink-0">
			<span class="text-gradient-cyan-magenta font-bold">Dewey — Docs</span>
		</div>

		if d.source.Get().err != nil {
			<div class="flex justify-center shrink-0">
				<span class="text-red font-bold">{fmt.Sprintf("failed to load doc: %v", d.source.Get().err)}</span>
			</div>
		} else {
			<div
				ref={d.contentRef}
				class="flex-col gap-1"
				flexGrow={1.0}
				scrollable={tui.ScrollVertical}
				scrollOffset={0, d.scrollY.Get()}
			>
				<markdown source={d.source.Get().content} />
			</div>
		}

		<div class="flex justify-center shrink-0">
			<span class="font-dim">↑/↓ or j/k or mouse wheel to scroll — q to quit</span>
		</div>
	</div>
}
