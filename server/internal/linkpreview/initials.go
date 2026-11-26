package linkpreview

import (
	"hash/crc32"
	"image"
	"image/color"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/math/fixed"
)

// Pastel backgrounds and dark lettering stay legible on light and dark Slack themes.
var initialsPalette = [...]struct{ background, ink color.RGBA }{
	{color.RGBA{255, 205, 213, 255}, color.RGBA{125, 28, 53, 255}},
	{color.RGBA{255, 215, 177, 255}, color.RGBA{124, 48, 13, 255}},
	{color.RGBA{251, 225, 145, 255}, color.RGBA{100, 65, 8, 255}},
	{color.RGBA{169, 235, 200, 255}, color.RGBA{16, 83, 60, 255}},
	{color.RGBA{154, 232, 220, 255}, color.RGBA{15, 80, 77, 255}},
	{color.RGBA{166, 229, 245, 255}, color.RGBA{14, 75, 99, 255}},
	{color.RGBA{183, 213, 251, 255}, color.RGBA{28, 63, 125, 255}},
	{color.RGBA{202, 207, 252, 255}, color.RGBA{56, 50, 128, 255}},
	{color.RGBA{226, 203, 251, 255}, color.RGBA{87, 43, 125, 255}},
	{color.RGBA{249, 197, 226, 255}, color.RGBA{121, 35, 88, 255}},
}

// Slack fetches a public image. The URL carries only the initials and a palette
// index; the sender's ID, name and email stay out of the request.
func InitialsAvatarURL(apiBase, initials, seed string) string {
	base, err := url.Parse(strings.TrimRight(apiBase, "/"))
	if err != nil || base.Host == "" || base.User != nil || base.Scheme != "https" || base.RawQuery != "" || base.Fragment != "" {
		return ""
	}
	base.Path = strings.TrimSuffix(base.Path, "/api") + "/api/link-previews/initials-v1.png"
	base.RawPath = ""
	index := crc32.ChecksumIEEE([]byte(seed)) % uint32(len(initialsPalette))
	base.RawQuery = url.Values{"i": {initials}, "c": {strconv.Itoa(int(index))}}.Encode()
	return base.String()
}

func (r *Renderer) InitialsAvatar(initials, swatch string) *image.RGBA {
	index, err := strconv.Atoi(swatch)
	if err != nil || index < 0 || index >= len(initialsPalette) || strconv.Itoa(index) != swatch || len(initials) == 0 || len(initials) > 2*utf8.UTFMax || !utf8.ValidString(initials) || utf8.RuneCountInString(initials) > 2 {
		return nil
	}
	for _, ch := range initials {
		if !unicode.IsGraphic(ch) || unicode.IsSpace(ch) {
			return nil
		}
	}
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	palette := initialsPalette[index]
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			coverage := min(1.0, max(0.0, 32.5-math.Hypot(float64(x)-31.5, float64(y)-31.5)))
			bg := palette.background
			img.Set(x, y, color.NRGBA{R: bg.R, G: bg.G, B: bg.B, A: uint8(255 * coverage)})
		}
	}
	p := r.painter(img)
	fontSize := 28
	if utf8.RuneCountInString(initials) == 1 {
		fontSize = 36
	}
	line := p.layout(initials, fontSize, size, 1, 700)
	if len(line.lines) > 0 {
		var advance fixed.Int26_6
		for _, run := range line.lines[0] {
			advance += run.Advance
		}
		line.width = advance.Ceil()
		p.paint(line, (size-line.width)/2, (size-line.height())/2+line.baselineOffset(), palette.ink, false)
	}
	return img
}
