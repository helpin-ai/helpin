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
	r.logo = image.NewRGBA(image.Rect(0, 0, 46, 46))
	icon.SetTarget(0, 0, 46, 46)
	icon.Draw(rasterx.NewDasher(46, 46, rasterx.NewScannerGV(46, 46, r.logo, r.logo.Bounds())), 1)
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
	fill(image.Rect(58, 44, 104, 90), green)
	p := r.painter(canvas)
	draw.Draw(canvas, image.Rect(58, 44, 104, 90), r.logo, image.Point{}, draw.Over)
	p.text("Helpin", 120, 80, 34, 360, 1, 650, ink)
	p.text(card.Type, 740, 77, 24, 402, 1, 500, muted)
	titleY := 230
	if card.Reference != "" {
		p.text(card.Type+" ID: "+card.Reference, 58, 160, 23, 1084, 1, 600, green)
	} else {
		titleY = 220
	}
	lines := p.text(card.Title, 58, titleY, 54, 1084, 3, 600, ink)
	descriptionY := titleY + lines*64 + 24
	if descriptionY <= 460 && card.Description != "" {
		p.text(card.Description, 58, descriptionY, 26, 1060, 2, 400, muted)
	}
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
	if value == "" {
		return 0
	}
	for _, face := range p.faces {
		face.SetVariations([]font.Variation{{Tag: opentype.MustNewTag("wght"), Value: float32(weight)}})
	}
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
	painter := render.Renderer{FontSize: float32(size), PixScale: 1, Color: ink}
	lineHeight := size * 120 / 100
	for i, line := range lines {
		sort.SliceStable(line, func(a, b int) bool { return line[a].VisualIndex < line[b].VisualIndex })
		left := x
		if direction == di.DirectionRTL {
			var advance fixed.Int26_6
			for _, run := range line {
				advance += run.Advance
			}
			left += width - advance.Ceil()
		}
		for _, run := range line {
			left = painter.DrawShapedRunAt(run, p.canvas, left, y+i*lineHeight)
		}
	}
	return len(lines)
}
