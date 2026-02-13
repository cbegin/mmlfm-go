package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cbegin/mmlfm-go"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	windowW      = 1100
	windowH      = 720
	minWindowW   = 980
	minWindowH   = 680
	uiSampleRate = 48000

	textScale = 2
	charW     = 7 * textScale
	lineH     = 14 * textScale
)

var (
	bgColor           = color.RGBA{192, 192, 192, 255}
	panelColor        = color.RGBA{192, 192, 192, 255}
	borderColor       = color.RGBA{128, 128, 128, 255}
	textColor         = color.RGBA{255, 255, 255, 255}
	buttonColor       = color.RGBA{192, 192, 192, 255}
	buttonPauseColor  = color.RGBA{192, 192, 192, 255}
	highlightColor    = color.RGBA{0, 0, 128, 255}
	editorPlaceholder = "Select an MML file to play."

	// 3D bevel colors for old-school embossed look.
	bevelLight  = color.RGBA{255, 255, 255, 255}
	bevelDarker = color.RGBA{64, 64, 64, 255}

	// Sunken panel / edit area interior.
	sunkenBgColor = color.RGBA{24, 24, 32, 255}

	// Slider fill accent.
	sliderFillColor = color.RGBA{0, 0, 128, 255}
)

const (
	fftSize    = 2048
	ringBufLen = 131072
)

type analyzer struct {
	mu          sync.Mutex
	sampleRate  int
	ring        []float32 // mono ring buffer
	writePos    int
	totalTapped int64 // total mono samples written since last reset
}

func newAnalyzer(sampleRate int) *analyzer {
	return &analyzer{
		sampleRate: sampleRate,
		ring:       make([]float32, ringBufLen),
	}
}

// Tap is called from the audio thread. Keep it minimal: just copy into ring.
func (a *analyzer) Tap(samples []float32) {
	a.mu.Lock()
	for i := 0; i+1 < len(samples); i += 2 {
		mono := (samples[i] + samples[i+1]) * 0.5
		a.ring[a.writePos] = mono
		a.writePos = (a.writePos + 1) % ringBufLen
		a.totalTapped++
	}
	a.mu.Unlock()
}

// Reset clears the tapped sample counter (call on new playback).
func (a *analyzer) Reset() {
	a.mu.Lock()
	a.totalTapped = 0
	a.mu.Unlock()
}

// Snapshot copies n samples aligned to what the listener actually hears.
// playbackPos is the audio driver's current output position in samples.
func (a *analyzer) Snapshot(n int, playbackPos int64) []float32 {
	if n > ringBufLen {
		n = ringBufLen
	}
	out := make([]float32, n)
	a.mu.Lock()
	// The delay is how far ahead the tap is from the speaker output.
	delay := int(a.totalTapped - playbackPos)
	if delay < 0 {
		delay = 0
	}
	if delay > ringBufLen-n {
		delay = ringBufLen - n
	}
	// Read from writePos - delay - n (i.e. what's playing now).
	start := (a.writePos - delay - n + ringBufLen*2) % ringBufLen
	for i := 0; i < n; i++ {
		out[i] = a.ring[(start+i)%ringBufLen]
	}
	a.mu.Unlock()
	return out
}

// fft computes a radix-2 FFT in-place.
func fft(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}
	// Bit-reversal permutation.
	bits := 0
	for m := n; m > 1; m >>= 1 {
		bits++
	}
	for i := 0; i < n; i++ {
		j := 0
		for b := 0; b < bits; b++ {
			if i&(1<<b) != 0 {
				j |= 1 << (bits - 1 - b)
			}
		}
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	// Cooley-Tukey iterative FFT.
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		wn := -2.0 * math.Pi / float64(size)
		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				t := cmplx.Rect(1, wn*float64(k)) * x[start+k+half]
				x[start+k+half] = x[start+k] - t
				x[start+k] = x[start+k] + t
			}
		}
	}
}

type navEntry struct {
	name  string
	path  string
	isDir bool
}

type game struct {
	player   *mmlfm.Player
	events   <-chan mmlfm.PlaybackEvent
	analyzer *analyzer
	scopeImg *ebiten.Image
	scopeW   int
	scopeH   int
	// Smoothed spectrum bins for display (log-magnitude, 0..1 range).
	specBins []float64
	wavePeak float64

	engineIdx int
	volume    float64
	octave    int
	eqGains   [5]float64 // 0..2 range, 1.0 = unity

	draggingVolume int // 0=none, 1=volume, 2=octave
	draggingEQ     int // -1=none, 0-4=band index

	editor       []rune
	editorScroll int
	wrappedLines []string
	wrapWidth    int
	wrapDirty    bool

	playing bool
	paused  bool

	status    string
	statusErr bool

	cwd       string
	nav       []navEntry
	navScroll int

	loadedPath string

	frameTick        int
	lastNavPath      string
	lastNavClickTick int

	textCache map[string]*ebiten.Image
	viewW     int
	viewH     int
}

var engineModes = []mmlfm.SynthMode{
	mmlfm.SynthModeFM,
	mmlfm.SynthModeChiptune,
	mmlfm.SynthModeNESAPU,
	mmlfm.SynthModeWavetable,
}

func newGame(initialText string, initialPath string) (*game, error) {
	a := newAnalyzer(uiSampleRate)
	pl, err := mmlfm.NewPlayer(uiSampleRate, mmlfm.WithLoopPlayback(false), mmlfm.WithSynthMode(engineModes[0]), mmlfm.WithSampleTap(a.Tap))
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if initialPath != "" {
		cwd = filepath.Dir(initialPath)
	}

	g := &game{
		player:       pl,
		events:       pl.Watch(),
		analyzer:     a,
		engineIdx:    0,
		volume:       1.0,
		eqGains:      [5]float64{1, 1, 1, 1, 1},
		draggingEQ:   -1,
		editor:       []rune(initialText),
		status:       "Ready",
		cwd:          cwd,
		loadedPath:   initialPath,
		textCache:    make(map[string]*ebiten.Image, 1024),
		editorScroll: 0,
		wrapDirty:    true,
		viewW:        windowW,
		viewH:        windowH,
	}
	if err := g.refreshNav(); err != nil {
		g.setError(err.Error())
	}
	return g, nil
}

func (g *game) Update() error {
	g.frameTick++
	g.pollEvents()
	g.handleMouse()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(bgColor)

	l := g.layoutRects()

	g.drawSunkenPanel(screen, l.nav)
	g.drawPanel(screen, l.eq)
	g.drawSunkenPanel(screen, l.editor)
	g.drawDarkPanel(screen, l.spectrum)
	g.drawButton(screen, l.play, g.playButtonLabel(), g.playButtonColor())
	g.drawButton(screen, l.engine, g.engineLabel(), buttonColor)
	g.drawOctaveSlider(screen, l.octave)
	g.drawVolumeSlider(screen, l.volume)
	g.drawSunkenPanel(screen, l.status)

	g.drawText(screen, "Files", l.nav.Min.X+8, l.nav.Min.Y+8)

	g.drawNavigator(screen, l.nav)
	g.drawEQ(screen, l.eq)
	g.drawEditor(screen, l.editor)
	g.drawSpectrum(screen, l.spectrum)
	g.drawStatus(screen, l.status)
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	if outsideW < minWindowW {
		outsideW = minWindowW
	}
	if outsideH < minWindowH {
		outsideH = minWindowH
	}
	g.viewW = outsideW
	g.viewH = outsideH
	return outsideW, outsideH
}
func (g *game) Close() { _ = g.player.Stop() }

func (g *game) pollEvents() {
	for {
		select {
		case ev, ok := <-g.events:
			if !ok {
				return
			}
			if ev.Kind == mmlfm.EventPlaybackEnded {
				g.playing = false
				g.paused = false
				if !g.statusErr {
					g.status = "Playback ended"
				}
			}
		default:
			return
		}
	}
}

func (g *game) handleMouse() {
	mx, my := ebiten.CursorPosition()
	l := g.layoutRects()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		switch {
		case pointInRect(mx, my, l.play):
			g.togglePlayPause()
			return
		case pointInRect(mx, my, l.engine):
			g.cycleEngine()
			return
		case pointInRect(mx, my, l.octave):
			g.draggingVolume = 2
			g.updateOctaveFromMouse(mx, l.octave)
			return
		case pointInRect(mx, my, l.volume):
			g.draggingVolume = 1
			g.updateVolumeFromMouse(mx, l.volume)
			return
		case pointInRect(mx, my, l.eq):
			g.clickEQ(mx, my, l.eq)
			return
		case pointInRect(mx, my, l.nav):
			g.clickNavigator(my, l.nav)
			return
		case pointInRect(mx, my, l.editor):
			g.clickEditorScroll(mx, my, l.editor)
		}
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.draggingVolume = 0
		g.draggingEQ = -1
	}
	if g.draggingVolume == 1 {
		g.updateVolumeFromMouse(mx, l.volume)
	}
	if g.draggingVolume == 2 {
		g.updateOctaveFromMouse(mx, l.octave)
	}
	if g.draggingEQ >= 0 {
		g.dragEQ(mx, my, l.eq)
	}

	_, wy := ebiten.Wheel()
	if wy == 0 {
		return
	}
	if pointInRect(mx, my, l.nav) {
		g.navScroll -= int(wy * 2)
		if g.navScroll < 0 {
			g.navScroll = 0
		}
	}
	if pointInRect(mx, my, l.editor) {
		g.editorScroll -= int(wy * 2)
		if g.editorScroll < 0 {
			g.editorScroll = 0
		}
	}
}

type uiLayout struct {
	nav, eq, editor, spectrum image.Rectangle
	play, engine, octave      image.Rectangle
	volume, status            image.Rectangle
}

func (g *game) layoutRects() uiLayout {
	w := g.viewW
	h := g.viewH
	if w < minWindowW {
		w = minWindowW
	}
	if h < minWindowH {
		h = minWindowH
	}

	pad := 20
	rowH := 44
	statusH := 40

	// Bottom: status row, then controls row above it.
	statusTop := h - pad - statusH
	controlsTop := statusTop - 8 - rowH

	// Left column: nav + EQ.
	navW := 280
	eqH := 120
	navBottom := controlsTop - 12
	eqTop := navBottom - eqH
	navRect := image.Rect(pad, pad, pad+navW, eqTop-8)
	eqRect := image.Rect(pad, eqTop, pad+navW, navBottom)

	// Right column: editor + spectrum.
	rightX := navRect.Max.X + 12
	rightW := w - rightX - pad
	if rightW < 320 {
		rightW = 320
	}
	contentBottom := controlsTop - 12
	contentH := contentBottom - pad
	scopeH := int(float64(contentH) * 0.28)
	if scopeH < 120 {
		scopeH = 120
	}
	if scopeH > 260 {
		scopeH = 260
	}
	editorRect := image.Rect(rightX, pad, rightX+rightW, contentBottom-scopeH-12)
	spectrumRect := image.Rect(rightX, editorRect.Max.Y+12, rightX+rightW, contentBottom)

	// Controls row.
	playRect := image.Rect(pad, controlsTop, pad+130, controlsTop+rowH)
	engineRect := image.Rect(pad+142, controlsTop, pad+350, controlsTop+rowH)
	octaveRect := image.Rect(pad+362, controlsTop, pad+600, controlsTop+rowH)
	volRight := pad + 612 + 260
	if volRight > w-pad {
		volRight = w - pad
	}
	volumeRect := image.Rect(pad+612, controlsTop, volRight, controlsTop+rowH)

	// Status row.
	statusRect := image.Rect(pad, statusTop, w-pad, statusTop+statusH)

	return uiLayout{
		nav: navRect, eq: eqRect, editor: editorRect, spectrum: spectrumRect,
		play: playRect, engine: engineRect, octave: octaveRect,
		volume: volumeRect, status: statusRect,
	}
}

func (g *game) drawNavigator(screen *ebiten.Image, rect image.Rectangle) {
	label := g.cwd
	if g.loadedPath != "" {
		label = g.cwd + "  [" + filepath.Base(g.loadedPath) + "]"
	}
	maxChars := max(8, (rect.Dx()-16)/charW)
	g.drawText(screen, shortenMiddle(label, maxChars), rect.Min.X+8, rect.Min.Y+8+lineH)

	top := rect.Min.Y + 12 + (lineH * 2)
	maxLines := (rect.Dy() - (lineH * 2) - 18) / lineH
	if maxLines < 1 {
		maxLines = 1
	}
	if g.navScroll > len(g.nav)-1 {
		g.navScroll = max(0, len(g.nav)-1)
	}

	for i := 0; i < maxLines; i++ {
		idx := g.navScroll + i
		if idx < 0 || idx >= len(g.nav) {
			break
		}
		entry := g.nav[idx]
		y := top + i*lineH
		if g.loadedPath != "" && !entry.isDir && samePath(entry.path, g.loadedPath) {
			gx := rect.Min.X + 6
			gy := y - 2
			gh := lineH + 2
			ebitenutil.DrawRect(screen, float64(gx), float64(gy), float64(rect.Dx()-12), float64(gh), highlightColor)
		}
		txt := entry.name
		if entry.isDir && entry.name != ".." {
			txt += "/"
		}
		g.drawText(screen, shortenEnd(txt, maxChars-1), rect.Min.X+10, y)
	}
}

func (g *game) drawEditor(screen *ebiten.Image, rect image.Rectangle) {
	text := string(g.editor)
	top := rect.Min.Y + 12 + lineH
	maxLines := (rect.Dy() - lineH - 20) / lineH
	if maxLines < 1 {
		maxLines = 1
	}
	maxChars := max(8, (rect.Dx()-24)/charW) // reserve space for scrollbar
	lines := g.wrappedEditorLines(maxChars)
	if g.editorScroll > len(lines)-1 {
		g.editorScroll = max(0, len(lines)-1)
	}
	maxScroll := max(0, len(lines)-maxLines)
	if g.editorScroll > maxScroll {
		g.editorScroll = maxScroll
	}

	if text == "" {
		g.drawText(screen, shortenEnd(editorPlaceholder, maxChars), rect.Min.X+8, top)
	}

	for i := 0; i < maxLines; i++ {
		idx := g.editorScroll + i
		if idx >= len(lines) {
			break
		}
		g.drawText(screen, shortenEnd(lines[idx], maxChars), rect.Min.X+8, top+i*lineH)
	}
	g.drawEditorScrollbar(screen, rect, top, maxLines, len(lines))
}

func (g *game) wrappedEditorLines(maxChars int) []string {
	if maxChars < 1 {
		maxChars = 1
	}
	if !g.wrapDirty && g.wrapWidth == maxChars && len(g.wrappedLines) > 0 {
		return g.wrappedLines
	}

	source := string(g.editor)
	baseLines := strings.Split(source, "\n")
	out := make([]string, 0, len(baseLines))
	for _, raw := range baseLines {
		if raw == "" {
			out = append(out, "")
			continue
		}
		rest := []rune(raw)
		for len(rest) > maxChars {
			cut := maxChars
			breakAt := cut
			for breakAt > 0 && rest[breakAt-1] != ' ' && rest[breakAt-1] != '\t' {
				breakAt--
			}
			if breakAt > maxChars/3 {
				cut = breakAt
			}
			line := strings.TrimRight(string(rest[:cut]), " \t")
			if line == "" {
				line = string(rest[:cut])
			}
			out = append(out, line)
			rest = []rune(strings.TrimLeft(string(rest[cut:]), " \t"))
		}
		out = append(out, string(rest))
	}
	if len(out) == 0 {
		out = append(out, "")
	}

	g.wrappedLines = out
	g.wrapWidth = maxChars
	g.wrapDirty = false
	return g.wrappedLines
}

func (g *game) drawEditorScrollbar(screen *ebiten.Image, rect image.Rectangle, top int, maxLines int, totalLines int) {
	trackX := rect.Max.X - 12
	trackY := top
	trackH := max(1, maxLines*lineH)
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), 6, float64(trackH), bevelDarker)

	if totalLines <= maxLines {
		return
	}
	maxScroll := totalLines - maxLines
	thumbH := max(lineH, int(float64(trackH)*float64(maxLines)/float64(totalLines)))
	thumbMaxY := trackH - thumbH
	thumbY := trackY
	if thumbMaxY > 0 && maxScroll > 0 {
		thumbY += int(float64(thumbMaxY) * float64(g.editorScroll) / float64(maxScroll))
	}
	thumbRect := image.Rect(trackX, thumbY, trackX+6, thumbY+thumbH)
	ebitenutil.DrawRect(screen, float64(trackX), float64(thumbY), 6, float64(thumbH), panelColor)
	drawBorder(screen, thumbRect)
}

func (g *game) clickEditorScroll(mx int, my int, rect image.Rectangle) {
	trackX := rect.Max.X - 14
	if mx < trackX {
		return
	}
	top := rect.Min.Y + 12 + lineH
	maxLines := (rect.Dy() - lineH - 20) / lineH
	if maxLines < 1 {
		maxLines = 1
	}
	maxChars := max(8, (rect.Dx()-24)/charW)
	totalLines := len(g.wrappedEditorLines(maxChars))
	if totalLines <= maxLines {
		g.editorScroll = 0
		return
	}
	maxScroll := totalLines - maxLines
	trackY := top
	trackH := max(1, maxLines*lineH)
	pos := clamp(float64(my-trackY), 0, float64(trackH))
	g.editorScroll = int((pos / float64(trackH)) * float64(maxScroll))
}

func (g *game) drawSpectrum(screen *ebiten.Image, rect image.Rectangle) {
	inner := image.Rect(rect.Min.X+8, rect.Min.Y+8, rect.Max.X-8, rect.Max.Y-8)
	width := inner.Dx()
	height := inner.Dy()
	if width <= 0 || height <= 0 {
		return
	}

	if g.scopeImg == nil || g.scopeW != width || g.scopeH != height {
		g.scopeW = width
		g.scopeH = height
		g.scopeImg = ebiten.NewImage(width, height)
	}

	// Clear with slight persistence for glow effect.
	g.scopeImg.Fill(color.RGBA{14, 16, 22, 255})

	// Grab latest samples from ring buffer.
	snap := g.analyzer.Snapshot(fftSize, g.player.PlaybackPosition())

	// --- Waveform (top 45%) ---
	waveH := int(float64(height) * 0.45)
	g.drawWaveform(g.scopeImg, snap, width, waveH)

	// Divider line.
	ebitenutil.DrawRect(g.scopeImg, 0, float64(waveH), float64(width), 1, color.RGBA{50, 54, 68, 180})

	// --- Spectrum analyzer (bottom 55%) ---
	specY := waveH + 1
	specH := height - specY
	g.drawSpectrumBars(g.scopeImg, snap, width, specH, specY)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(inner.Min.X), float64(inner.Min.Y))
	screen.DrawImage(g.scopeImg, op)

}

func (g *game) drawWaveform(dst *ebiten.Image, samples []float32, width int, height int) {
	if len(samples) < 2 || width < 2 || height < 4 {
		return
	}
	midY := height / 2

	// Center line.
	ebitenutil.DrawRect(dst, 0, float64(midY), float64(width), 1, color.RGBA{40, 44, 58, 100})

	// Auto-gain: track peak with fast attack, slow release.
	peak := float32(0)
	for _, s := range samples {
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	target := float64(peak)
	if target < 0.01 {
		target = 0.01
	}
	if target > g.wavePeak {
		g.wavePeak = g.wavePeak*0.3 + target*0.7
	} else {
		g.wavePeak = g.wavePeak*0.995 + target*0.005
	}
	if g.wavePeak < 0.01 {
		g.wavePeak = 0.01
	}
	gain := float64(midY-2) / g.wavePeak

	// Draw the waveform, downsampling to pixel width.
	// Use zero-crossing trigger to stabilize the display.
	triggerOffset := findZeroCrossing(samples, len(samples)/4)
	visible := len(samples) - triggerOffset
	if visible < 2 {
		visible = 2
	}

	waveColor := color.RGBA{80, 200, 255, 220}
	prevX := 0
	prevY := midY - int(float64(samples[triggerOffset])*gain)
	for px := 1; px < width; px++ {
		si := triggerOffset + px*visible/width
		if si >= len(samples) {
			si = len(samples) - 1
		}
		y := midY - int(float64(samples[si])*gain)
		ebitenutil.DrawLine(dst, float64(prevX), float64(prevY), float64(px), float64(y), waveColor)
		prevX = px
		prevY = y
	}
}

// findZeroCrossing finds a rising zero-crossing in samples to stabilize the waveform display.
func findZeroCrossing(samples []float32, searchLen int) int {
	if searchLen > len(samples)-2 {
		searchLen = len(samples) - 2
	}
	for i := 1; i < searchLen; i++ {
		if samples[i-1] <= 0 && samples[i] > 0 {
			return i
		}
	}
	return 0
}

func (g *game) drawSpectrumBars(dst *ebiten.Image, samples []float32, width int, height int, yOffset int) {
	if len(samples) < fftSize || width < 4 || height < 4 {
		return
	}

	// Apply Hann window and build complex input.
	buf := make([]complex128, fftSize)
	for i := 0; i < fftSize; i++ {
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(fftSize-1)))
		buf[i] = complex(float64(samples[len(samples)-fftSize+i])*w, 0)
	}
	fft(buf)

	// Convert to log-magnitude, mapped to display bins.
	// Use log-frequency scale: map pixel columns to FFT bins logarithmically.
	numBars := width / 3
	if numBars < 16 {
		numBars = 16
	}
	if numBars > 256 {
		numBars = 256
	}

	// Ensure our smoothing buffer is the right size.
	if len(g.specBins) != numBars {
		g.specBins = make([]float64, numBars)
	}

	halfFFT := fftSize / 2
	minBin := 1                                             // skip DC
	maxBin := halfFFT * 18000 / (g.analyzer.sampleRate / 2) // up to ~18kHz
	if maxBin > halfFFT {
		maxBin = halfFFT
	}
	logMin := math.Log(float64(minBin))
	logMax := math.Log(float64(maxBin))

	for i := 0; i < numBars; i++ {
		// Log-frequency mapping.
		frac0 := float64(i) / float64(numBars)
		frac1 := float64(i+1) / float64(numBars)
		binStart := int(math.Exp(logMin + frac0*(logMax-logMin)))
		binEnd := int(math.Exp(logMin + frac1*(logMax-logMin)))
		if binEnd <= binStart {
			binEnd = binStart + 1
		}
		if binEnd > halfFFT {
			binEnd = halfFFT
		}

		// Average magnitude in this range.
		sum := 0.0
		for b := binStart; b < binEnd; b++ {
			mag := cmplx.Abs(buf[b])
			sum += mag
		}
		avg := sum / float64(binEnd-binStart)

		// Convert to dB, normalize to 0..1 range (~-80dB to 0dB).
		db := 20.0 * math.Log10(avg/float64(fftSize)+1e-10)
		norm := (db + 80.0) / 80.0
		if norm < 0 {
			norm = 0
		}
		if norm > 1 {
			norm = 1
		}

		// Smooth: fast attack, slower decay.
		prev := g.specBins[i]
		if norm > prev {
			g.specBins[i] = prev*0.3 + norm*0.7
		} else {
			g.specBins[i] = prev*0.85 + norm*0.15
		}
	}

	// Draw bars.
	barW := float64(width) / float64(numBars)
	for i := 0; i < numBars; i++ {
		v := g.specBins[i]
		barH := v * float64(height-4)
		if barH < 1 {
			barH = 1
		}
		x := float64(i) * barW
		y := float64(yOffset) + float64(height-2) - barH

		// Color gradient: blue at bottom -> green at mid -> orange/red at top.
		r, gr, b := spectrumColor(v)
		col := color.RGBA{r, gr, b, 220}
		ebitenutil.DrawRect(dst, x+1, y, barW-1, barH, col)
	}
}

func spectrumColor(v float64) (uint8, uint8, uint8) {
	if v < 0.33 {
		t := v / 0.33
		return uint8(30 + 20*t), uint8(80 + 120*t), uint8(200 + 55*t)
	}
	if v < 0.66 {
		t := (v - 0.33) / 0.33
		return uint8(50 + 140*t), uint8(200 + 30*t), uint8(255 - 100*t)
	}
	t := (v - 0.66) / 0.34
	return uint8(190 + 65*t), uint8(230 - 100*t), uint8(155 - 100*t)
}

func (g *game) drawStatus(screen *ebiten.Image, rect image.Rectangle) {
	msg := "Status: " + g.status
	if g.statusErr {
		msg = "Status: ERROR - " + g.status
	}
	maxChars := max(8, (rect.Dx()-16)/charW)
	g.drawText(screen, shortenEnd(msg, maxChars), rect.Min.X+8, rect.Min.Y+6)
}

func (g *game) drawVolumeSlider(screen *ebiten.Image, rect image.Rectangle) {
	g.drawPanel(screen, rect)
	label := fmt.Sprintf("Vol %d%%", int(g.volume*100+0.5))
	g.drawText(screen, label, rect.Min.X+8, rect.Min.Y+8)

	trackX := rect.Min.X + 130
	trackW := rect.Dx() - 146
	trackY := rect.Min.Y + rect.Dy()/2 - 4
	if trackW < 20 {
		return
	}
	// Sunken track groove.
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), float64(trackW), 8, bevelDarker)
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), float64(trackW-1), 1, borderColor)
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), 1, 7, borderColor)
	// Fill.
	fillW := int(float64(trackW) * clamp(g.volume, 0, 1))
	if fillW > 2 {
		ebitenutil.DrawRect(screen, float64(trackX+1), float64(trackY+1), float64(fillW-1), 6, sliderFillColor)
	}
	// Raised knob.
	knobX := trackX + fillW - 5
	if knobX < trackX-5 {
		knobX = trackX - 5
	}
	if knobX > trackX+trackW-5 {
		knobX = trackX + trackW - 5
	}
	knobRect := image.Rect(knobX, trackY-4, knobX+10, trackY+12)
	ebitenutil.DrawRect(screen, float64(knobRect.Min.X), float64(knobRect.Min.Y), float64(knobRect.Dx()), float64(knobRect.Dy()), panelColor)
	drawBorder(screen, knobRect)
}

func (g *game) clickNavigator(my int, rect image.Rectangle) {
	top := rect.Min.Y + 12 + (lineH * 2)
	row := (my - top) / lineH
	if row < 0 {
		return
	}
	idx := g.navScroll + row
	if idx < 0 || idx >= len(g.nav) {
		return
	}
	entry := g.nav[idx]
	if entry.isDir {
		g.cwd = entry.path
		g.navScroll = 0
		if err := g.refreshNav(); err != nil {
			g.setError(err.Error())
			return
		}
		g.setStatus("Directory: " + g.cwd)
		return
	}

	doubleClickSame := samePath(entry.path, g.lastNavPath) && (g.frameTick-g.lastNavClickTick) <= 18
	g.lastNavPath = entry.path
	g.lastNavClickTick = g.frameTick

	if err := g.loadFile(entry.path); err != nil {
		g.setError(err.Error())
		return
	}
	if doubleClickSame {
		g.restartPlayback()
		return
	}
	g.setStatus("Loaded " + filepath.Base(entry.path))
}

func (g *game) refreshNav() error {
	items, err := os.ReadDir(g.cwd)
	if err != nil {
		return err
	}
	dirs := make([]navEntry, 0)
	files := make([]navEntry, 0)

	parent := filepath.Dir(g.cwd)
	if parent != g.cwd {
		dirs = append(dirs, navEntry{name: "..", path: parent, isDir: true})
	}

	for _, it := range items {
		name := it.Name()
		full := filepath.Join(g.cwd, name)
		if it.IsDir() {
			dirs = append(dirs, navEntry{name: name, path: full, isDir: true})
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".mml") {
			files = append(files, navEntry{name: name, path: full, isDir: false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].name == ".." {
			return true
		}
		if dirs[j].name == ".." {
			return false
		}
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})
	g.nav = append(dirs, files...)
	return nil
}

func (g *game) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_ = g.player.Stop()
	g.playing = false
	g.paused = false

	g.editor = []rune(string(data))
	g.editorScroll = 0
	g.wrapDirty = true
	g.loadedPath = path
	g.cwd = filepath.Dir(path)

	return g.refreshNav()
}

func (g *game) cycleEngine() {
	wasPlaying := g.playing
	g.engineIdx = (g.engineIdx + 1) % len(engineModes)
	if err := g.rebuildPlayer(); err != nil {
		g.setError(err.Error())
		return
	}
	if wasPlaying {
		g.restartPlayback()
		return
	}
	g.setStatus("Engine: " + g.engineLabel())
}

func (g *game) rebuildPlayer() error {
	if g.player != nil {
		_ = g.player.Stop()
	}
	pl, err := mmlfm.NewPlayer(
		uiSampleRate,
		mmlfm.WithLoopPlayback(false),
		mmlfm.WithSynthMode(engineModes[g.engineIdx]),
		mmlfm.WithSampleTap(g.analyzer.Tap),
	)
	if err != nil {
		return err
	}
	pl.SetMasterVolume(g.volume)
	g.player = pl
	g.events = pl.Watch()
	g.playing = false
	g.paused = false
	return nil
}

func (g *game) engineLabel() string {
	switch engineModes[g.engineIdx] {
	case mmlfm.SynthModeFM:
		return "FM"
	case mmlfm.SynthModeChiptune:
		return "Chiptune"
	case mmlfm.SynthModeNESAPU:
		return "NESAPU"
	case mmlfm.SynthModeWavetable:
		return "Wavetable"
	default:
		return string(engineModes[g.engineIdx])
	}
}

func (g *game) updateVolumeFromMouse(mx int, rect image.Rectangle) {
	trackX := rect.Min.X + 130
	trackW := rect.Dx() - 146
	if trackW <= 0 {
		return
	}
	v := clamp(float64(mx-trackX)/float64(trackW), 0, 1)
	g.volume = v
	if g.player != nil {
		g.player.SetMasterVolume(v)
	}
	g.setStatus(fmt.Sprintf("Volume: %d%%", int(v*100+0.5)))
}

const (
	octaveMin = -3
	octaveMax = 3
)

func (g *game) drawOctaveSlider(screen *ebiten.Image, rect image.Rectangle) {
	g.drawPanel(screen, rect)
	label := fmt.Sprintf("Oct %+d", g.octave)
	g.drawText(screen, label, rect.Min.X+8, rect.Min.Y+8)

	trackX := rect.Min.X + 100
	trackW := rect.Dx() - 116
	trackY := rect.Min.Y + rect.Dy()/2 - 4
	if trackW < 20 {
		return
	}
	// Sunken track groove.
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), float64(trackW), 8, bevelDarker)
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), float64(trackW-1), 1, borderColor)
	ebitenutil.DrawRect(screen, float64(trackX), float64(trackY), 1, 7, borderColor)

	// Draw center mark.
	centerX := trackX + trackW/2
	ebitenutil.DrawRect(screen, float64(centerX)-1, float64(trackY-2), 2, 12, borderColor)

	// Knob position: map octave range to track.
	frac := float64(g.octave-octaveMin) / float64(octaveMax-octaveMin)
	knobX := trackX + int(frac*float64(trackW)) - 5
	if knobX < trackX-5 {
		knobX = trackX - 5
	}
	if knobX > trackX+trackW-5 {
		knobX = trackX + trackW - 5
	}
	knobRect := image.Rect(knobX, trackY-4, knobX+10, trackY+12)
	ebitenutil.DrawRect(screen, float64(knobRect.Min.X), float64(knobRect.Min.Y), float64(knobRect.Dx()), float64(knobRect.Dy()), panelColor)
	drawBorder(screen, knobRect)
}

func (g *game) updateOctaveFromMouse(mx int, rect image.Rectangle) {
	trackX := rect.Min.X + 100
	trackW := rect.Dx() - 116
	if trackW <= 0 {
		return
	}
	frac := clamp(float64(mx-trackX)/float64(trackW), 0, 1)
	oct := int(math.Round(frac*float64(octaveMax-octaveMin))) + octaveMin
	if oct < octaveMin {
		oct = octaveMin
	}
	if oct > octaveMax {
		oct = octaveMax
	}
	if oct != g.octave {
		g.octave = oct
		g.player.SetTranspose(oct)
		if g.playing {
			g.restartPlayback()
		}
	}
	g.setStatus(fmt.Sprintf("Octave: %+d", g.octave))
}

var eqBandLabels = [5]string{"Lo", "LoM", "Mid", "HiM", "Hi"}

func (g *game) drawEQ(screen *ebiten.Image, rect image.Rectangle) {
	numBands := 5
	pad := 8
	labelH := 4
	innerX := rect.Min.X + pad
	innerW := rect.Dx() - pad*2
	innerY := rect.Min.Y + labelH
	innerH := rect.Dy() - labelH - pad

	bandW := innerW / numBands
	if bandW < 10 {
		return
	}

	for i := 0; i < numBands; i++ {
		bx := innerX + i*bandW
		by := innerY
		bw := bandW - 4
		bh := innerH

		// Sunken track groove.
		ebitenutil.DrawRect(screen, float64(bx+bw/2-2), float64(by), 4, float64(bh), bevelDarker)

		// Center line (gain = 1.0).
		centerY := by + bh/2
		ebitenutil.DrawRect(screen, float64(bx), float64(centerY), float64(bw), 1, borderColor)

		// Knob: map gain 0..2 to bottom..top.
		frac := clamp(g.eqGains[i]/2.0, 0, 1)
		knobY := by + bh - int(frac*float64(bh)) - 4

		// Raised 3D knob.
		knobRect := image.Rect(bx+2, knobY, bx+bw-2, knobY+8)
		ebitenutil.DrawRect(screen, float64(knobRect.Min.X), float64(knobRect.Min.Y), float64(knobRect.Dx()), float64(knobRect.Dy()), panelColor)
		drawBorder(screen, knobRect)
	}
}

func (g *game) clickEQ(mx, my int, rect image.Rectangle) {
	band := g.eqBandFromMouse(mx, rect)
	if band < 0 {
		return
	}
	g.draggingEQ = band
	g.dragEQ(mx, my, rect)
}

func (g *game) dragEQ(mx, my int, rect image.Rectangle) {
	band := g.draggingEQ
	if band < 0 || band >= 5 {
		return
	}
	pad := 8
	labelH := 4
	innerY := rect.Min.Y + labelH
	innerH := rect.Dy() - labelH - pad
	if innerH <= 0 {
		return
	}
	// Map y position to gain: top = 2.0, bottom = 0.0.
	frac := 1.0 - clamp(float64(my-innerY)/float64(innerH), 0, 1)
	gain := frac * 2.0
	g.eqGains[band] = gain
	g.player.SetEQBand(band, float32(gain))
	g.setStatus(fmt.Sprintf("EQ %s: %.1f", eqBandLabels[band], gain))
}

func (g *game) eqBandFromMouse(mx int, rect image.Rectangle) int {
	pad := 8
	innerX := rect.Min.X + pad
	innerW := rect.Dx() - pad*2
	numBands := 5
	bandW := innerW / numBands
	if bandW <= 0 {
		return -1
	}
	idx := (mx - innerX) / bandW
	if idx < 0 || idx >= numBands {
		return -1
	}
	return idx
}

func (g *game) togglePlayPause() {
	if !g.playing {
		g.restartPlayback()
		return
	}
	if g.paused {
		g.player.Resume()
		g.paused = false
		g.setStatus("Playing")
		return
	}
	g.player.Pause()
	g.paused = true
	g.setStatus("Paused")
}

func (g *game) restartPlayback() {
	text := strings.TrimSpace(string(g.editor))
	if text == "" {
		g.setError("Editor is empty")
		return
	}
	g.analyzer.Reset()
	if err := g.player.PlayMML(text); err != nil {
		g.playing = false
		g.paused = false
		g.setError(err.Error())
		return
	}
	g.playing = true
	g.paused = false
	g.player.SetMasterVolume(g.volume)
	g.setStatus("Playing")
}

func (g *game) playButtonLabel() string {
	if !g.playing {
		return "Play"
	}
	if g.paused {
		return "Resume"
	}
	return "Pause"
}

func (g *game) playButtonColor() color.Color {
	if g.playing && !g.paused {
		return buttonPauseColor
	}
	return buttonColor
}

func (g *game) setError(msg string) {
	g.status = msg
	g.statusErr = true
}

func (g *game) setStatus(msg string) {
	g.status = msg
	g.statusErr = false
}

func (g *game) drawPanel(screen *ebiten.Image, rect image.Rectangle) {
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), panelColor)
	drawBorder(screen, rect)
}

func (g *game) drawSunkenPanel(screen *ebiten.Image, rect image.Rectangle) {
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), sunkenBgColor)
	drawSunkenBorder(screen, rect)
}

func (g *game) drawDarkPanel(screen *ebiten.Image, rect image.Rectangle) {
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), color.RGBA{0, 0, 0, 255})
	drawSunkenBorder(screen, rect)
}

func (g *game) drawButton(screen *ebiten.Image, rect image.Rectangle, label string, fill color.Color) {
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), panelColor)
	drawBorder(screen, rect)
	labelW := len([]rune(label)) * charW
	x := rect.Min.X + (rect.Dx()-labelW)/2
	y := rect.Min.Y + (rect.Dy()-lineH)/2
	g.drawText(screen, label, x, y)
}

// drawBorder draws a raised 3D bevel (highlight top/left, shadow bottom/right).
func drawBorder(screen *ebiten.Image, rect image.Rectangle) {
	x := float64(rect.Min.X)
	y := float64(rect.Min.Y)
	w := float64(rect.Dx())
	h := float64(rect.Dy())
	// Outer highlight: top and left.
	ebitenutil.DrawRect(screen, x, y, w-1, 1, bevelLight)
	ebitenutil.DrawRect(screen, x, y+1, 1, h-2, bevelLight)
	// Outer shadow: bottom and right.
	ebitenutil.DrawRect(screen, x, y+h-1, w, 1, bevelDarker)
	ebitenutil.DrawRect(screen, x+w-1, y, 1, h, bevelDarker)
	// Inner shadow: bottom and right.
	ebitenutil.DrawRect(screen, x+1, y+h-2, w-3, 1, borderColor)
	ebitenutil.DrawRect(screen, x+w-2, y+1, 1, h-3, borderColor)
}

// drawSunkenBorder draws a sunken 3D bevel (shadow top/left, highlight bottom/right).
func drawSunkenBorder(screen *ebiten.Image, rect image.Rectangle) {
	x := float64(rect.Min.X)
	y := float64(rect.Min.Y)
	w := float64(rect.Dx())
	h := float64(rect.Dy())
	// Outer shadow: top and left.
	ebitenutil.DrawRect(screen, x, y, w-1, 1, borderColor)
	ebitenutil.DrawRect(screen, x, y+1, 1, h-2, borderColor)
	// Outer highlight: bottom and right.
	ebitenutil.DrawRect(screen, x, y+h-1, w, 1, bevelLight)
	ebitenutil.DrawRect(screen, x+w-1, y, 1, h, bevelLight)
	// Inner shadow: top and left.
	ebitenutil.DrawRect(screen, x+1, y+1, w-3, 1, bevelDarker)
	ebitenutil.DrawRect(screen, x+1, y+2, 1, h-4, bevelDarker)
}

func (g *game) drawText(screen *ebiten.Image, msg string, x int, y int) {
	if msg == "" {
		return
	}
	img := g.textCache[msg]
	if img == nil {
		w := max(1, len([]rune(msg))*7)
		img = ebiten.NewImage(w, 14)
		ebitenutil.DebugPrintAt(img, msg, 0, 0)
		if len(g.textCache) > 3000 {
			g.textCache = make(map[string]*ebiten.Image, 1024)
		}
		g.textCache[msg] = img
	}
	// Embossed shadow (dark offset behind text).
	opS := &ebiten.DrawImageOptions{}
	opS.GeoM.Scale(textScale, textScale)
	opS.GeoM.Translate(float64(x+2), float64(y+2))
	opS.ColorScale.Scale(0, 0, 0, 1)
	screen.DrawImage(img, opS)
	// Main text (white).
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(textScale, textScale)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func shortenEnd(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars <= 3 {
		return string(r[:max(0, maxChars)])
	}
	return string(r[:maxChars-3]) + "..."
}

func shortenMiddle(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars <= 7 {
		return shortenEnd(s, maxChars)
	}
	left := (maxChars - 3) / 2
	right := maxChars - 3 - left
	return string(r[:left]) + "..." + string(r[len(r)-right:])
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func pointInRect(x, y int, rect image.Rectangle) bool {
	return x >= rect.Min.X && x < rect.Max.X && y >= rect.Min.Y && y < rect.Max.Y
}

func main() {
	var (
		initialText string
		initialPath string
	)
	if len(os.Args) > 1 {
		p, err := filepath.Abs(os.Args[1])
		if err != nil {
			log.Fatalf("resolve %q: %v", os.Args[1], err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			log.Fatalf("read %q: %v", p, err)
		}
		initialText = string(data)
		initialPath = p
	}

	g, err := newGame(initialText, initialPath)
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	ebiten.SetWindowSize(windowW, windowH)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(minWindowW, minWindowH, -1, -1)
	ebiten.SetWindowTitle(fmt.Sprintf("mmlfm-go player"))
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
