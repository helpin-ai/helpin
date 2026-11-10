package linkpreview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sort"

	"github.com/go-text/render"
	"github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sync/singleflight"

	"github.com/helpin-ai/helpin/server/internal/model"
)

//go:embed fonts/*.ttf
var previewFonts embed.FS

// The unboxed ink mark matches frontend/public/brand/helpin-icon-ink.svg.
//
//go:embed assets/helpin-icon.svg
var previewLogo []byte

// Renderer draws the common 1200×630 preview template using local font assets.
// Only rendered bytes are cached; callers must resolve eligibility first.
type Renderer struct {
	fonts  []*font.Font
	cache  *lru.Cache[[32]byte, []byte]
	flight singleflight.Group
	slots  chan struct{}
	logo   *image.RGBA
}

// NewRenderer parses bundled fonts once and limits the raster cache to 64 cards.
func NewRenderer() (*Renderer, error) {
	cache, err := lru.New[[32]byte, []byte](64)
	if err != nil {
		return nil, err
	}
	r := &Renderer{cache: cache, slots: make(chan struct{}, 2)}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(previewLogo))
	if err != nil {
		return nil, fmt.Errorf("parse preview logo: %w", err)
	}
	r.logo = image.NewRGBA(image.Rect(0, 0, 48, 48))
	// Scale the viewBox translation too: the brand asset has a nonzero origin.
	icon.Transform = rasterx.Identity.Scale(48/icon.ViewBox.W, 48/icon.ViewBox.H).Translate(-icon.ViewBox.X, -icon.ViewBox.Y)
	icon.Draw(rasterx.NewDasher(48, 48, rasterx.NewScannerGV(48, 48, r.logo, r.logo.Bounds())), 1)
	for _, name := range []string{"Inter", "NotoSansArabic", "NotoSansDevanagari", "NotoSansSC", "NotoEmoji"} {
		data, err := previewFonts.ReadFile("fonts/" + name + ".ttf")
		if err != nil {
			return nil, fmt.Errorf("read preview font: %w", err)
		}
		face, err := font.ParseTTF(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse preview font %s: %w", name, err)
		}
		r.fonts = append(r.fonts, face.Font)
	}
	return r, nil
}

// PNG returns an image for an already-authorized, normalized card. It never
// loads URLs, fonts, workspace assets, or other remote resources while drawing.
func (r *Renderer) PNG(ctx context.Context, card model.LinkPreview) ([]byte, error) {
	card.URL, card.ImageURL = "", ""
	data, err := json.Marshal(card)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(data)
	if png, ok := r.cache.Get(key); ok {
		return png, nil
	}
	result := r.flight.DoChan(fmt.Sprintf("%x", key), func() (value any, err error) {
		// singleflight renders outside the HTTP recovery middleware. Keep a
		// shaping/rasterization panic from terminating the API process.
		defer func() {
			if recovered := recover(); recovered != nil {
				value, err = nil, fmt.Errorf("preview renderer panic (%T)", recovered)
			}
		}()
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if png, ok := r.cache.Get(key); ok {
			return png, nil
		}
		canvas := r.draw(card)
		var out bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(&out, canvas); err != nil {
			return nil, err
		}
		data := out.Bytes()
		// Keep the cache under 16 MiB even if a card compresses unusually poorly.
		if len(data) <= 256*1024 {
			r.cache.Add(key, data)
		}
		return data, nil
	})
	select {
	case value := <-result:
		if value.Err != nil {
			return nil, value.Err
		}
		return value.Val.([]byte), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Renderer) draw(card model.LinkPreview) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	fill := func(rect image.Rectangle, c color.Color) {
		draw.Draw(canvas, rect, image.NewUniform(c), image.Point{}, draw.Src)
	}
	bg := color.RGBA{250, 248, 241, 255}
	ink := color.RGBA{30, 38, 33, 255}
	muted := color.RGBA{76, 85, 77, 255}
	green := color.RGBA{38, 79, 55, 255}
	fill(canvas.Bounds(), bg)
	fill(image.Rect(0, 0, 1200, 8), green)
	p := r.painter(canvas)
	draw.Draw(canvas, image.Rect(58, 44, 106, 92), r.logo, image.Point{}, draw.Over)
	// Preserve the app lockup's 3:2 icon/text ratio, semibold wordmark,
	// small gap, and -0.04em letter spacing.
	wordmark := p.layout("Helpin", 32, 360, 1, 600)
	if len(wordmark.lines) > 0 {
		shaping.AddSpacing(wordmark.lines[0], []rune("Helpin"), 0, -fixed.I(32)*4/100)
	}
	p.paint(wordmark, 111, 80, color.RGBA{36, 36, 36, 255}, false)
	p.paint(p.layout(card.Type, 24, 402, 1, 500), 740, 77, muted, true)

	// Measure the whole content block before positioning it. The reference,
	// title and description stay together and centered between header/footer,
	// whether the title takes one, two or three lines.
	title := p.layout(card.Title, 54, 1084, 3, 600)
	description := p.layout(card.Description, 26, 1060, 2, 400)
	var reference textBlock
	if card.Reference != "" {
		reference = p.layout(card.Type+" ID: "+card.Reference, 23, 1084, 1, 600)
	}
	blockHeight := title.height()
	if reference.height() > 0 {
		blockHeight += reference.height() + 16
	}
	if description.height() > 0 {
		blockHeight += 20 + description.height()
	}
	const bodyTop, bodyBottom = 124, 475
	y := bodyTop + (bodyBottom-bodyTop-blockHeight)/2
	if reference.height() > 0 {
		p.paint(reference, 58, y+reference.baselineOffset(), green, false)
		y += reference.height() + 16
	}
	p.paint(title, 58, y+title.baselineOffset(), ink, false)
	y += title.height() + 20
	p.paint(description, 58, y+description.baselineOffset(), muted, false)
	fill(image.Rect(58, 507, 1142, 509), color.RGBA{207, 213, 201, 255})
	if len(card.Fields) == 0 {
		p.text(Tagline, 58, 558, 25, 1084, 2, 400, muted)
	} else {
		for i, f := range card.Fields {
			x := 58 + i*370
			p.text(f.Label, x, 546, 21, 330, 1, 500, muted)
			p.text(f.Value, x, 585, 28, 330, 1, 600, ink)
		}
	}
	return canvas
}

type cardPainter struct {
	canvas *image.RGBA
	faces  []*font.Face
}

func (r *Renderer) painter(canvas *image.RGBA) *cardPainter {
	p := &cardPainter{canvas: canvas}
	// Faces own mutable shaping caches and must not be shared across requests.
	for _, f := range r.fonts {
		p.faces = append(p.faces, font.NewFace(f))
	}
	return p
}

type previewFontMap []*font.Face

func (faces previewFontMap) ResolveFace(r rune) *font.Face {
	for _, face := range faces {
		if _, ok := face.NominalGlyph(r); ok {
			return face
		}
	}
	return faces[0]
}

func (p *cardPainter) text(value string, x, y, size, width, maxLines, weight int, ink color.Color) int {
	block := p.layout(value, size, width, maxLines, weight)
	p.paint(block, x, y, ink, false)
	return len(block.lines)
}

type textBlock struct {
	lines     []shaping.Line
	direction di.Direction
	size      int
	width     int
	weight    int
}

func (b textBlock) height() int { return len(b.lines) * (b.size * 120 / 100) }

// Match a CSS line box: font ascent plus half the remaining line leading.
func (b textBlock) baselineOffset() int {
	if len(b.lines) == 0 {
		return 0
	}
	var ascent, descent fixed.Int26_6
	for _, run := range b.lines[0] {
		ascent = max(ascent, run.LineBounds.Ascent)
		descent = min(descent, run.LineBounds.Descent)
	}
	leading := fixed.I(b.size*120/100) - (ascent - descent)
	return (ascent + leading/2).Round()
}

func (p *cardPainter) setWeight(weight int) {
	for _, face := range p.faces {
		face.SetVariations([]font.Variation{{Tag: opentype.MustNewTag("wght"), Value: float32(weight)}})
	}
}

func (p *cardPainter) layout(value string, size, width, maxLines, weight int) textBlock {
	block := textBlock{size: size, width: width, weight: weight}
	if value == "" {
		return block
	}
	p.setWeight(weight)
	chars := []rune(value)
	direction := di.DirectionLTR
	var paragraph bidi.Paragraph
	bidiRuns := paragraph.Segment(chars, bidi.LeftToRight)
	if bidiRuns.NumRuns() > 0 && !bidiRuns.Run(0).IsLeftToRight() {
		direction = di.DirectionRTL
	}
	input := shaping.Input{Text: chars, RunEnd: len(chars), Size: fixed.I(size), Direction: direction}
	var segmenter shaping.Segmenter
	inputs := segmenter.Split(input, previewFontMap(p.faces))
	var shaper shaping.HarfbuzzShaper
	outputs := make([]shaping.Output, len(inputs))
	for i, run := range inputs {
		outputs[i] = shaper.Shape(run)
	}
	ellipsis := []rune("…")
	config := shaping.WrapConfig{Direction: direction, TruncateAfterLines: maxLines}.WithTruncator(&shaper, shaping.Input{Text: ellipsis, RunEnd: 1, Face: p.faces[0], Size: fixed.I(size), Direction: direction})
	var wrapper shaping.LineWrapper
	lines, _ := wrapper.WrapParagraph(config, width, chars, shaping.NewSliceIterator(outputs))
	block.lines, block.direction = lines, direction
	return block
}

func (p *cardPainter) paint(block textBlock, x, y int, ink color.Color, alignRight bool) {
	if len(block.lines) == 0 {
		return
	}
	// Layouts retain faces. Restore their weight after measuring other styles.
	p.setWeight(block.weight)
	painter := render.Renderer{FontSize: float32(block.size), PixScale: 1, Color: ink}
	lineHeight := block.size * 120 / 100
	for i, line := range block.lines {
		sort.SliceStable(line, func(a, b int) bool { return line[a].VisualIndex < line[b].VisualIndex })
		left := x
		if alignRight || block.direction == di.DirectionRTL {
			var advance fixed.Int26_6
			for _, run := range line {
				advance += run.Advance
			}
			left += block.width - advance.Ceil()
		}
		for _, run := range line {
			left = painter.DrawShapedRunAt(run, p.canvas, left, y+i*lineHeight)
		}
	}
}
