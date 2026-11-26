package linkpreview

import (
	"net/url"
	"testing"
)

func TestInitialsAvatars(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	colors := map[string]bool{}
	for _, seed := range []string{"maya", "kit", "jo", "sam", "ivy"} {
		raw := InitialsAvatarURL("https://api.example", "李王", seed)
		u, err := url.Parse(raw)
		if err != nil || u.Path != "/api/link-previews/initials-v1.png" {
			t.Fatalf("URL %q, %v", raw, err)
		}
		q := u.Query()
		if q.Get("i") != "李王" {
			t.Fatalf("initials lost: %v", q)
		}
		colors[q.Get("c")] = true
		img := r.InitialsAvatar(q.Get("i"), q.Get("c"))
		if img == nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
			t.Fatal("missing square avatar")
		}
		bg := img.RGBAAt(32, 4)
		ink := 0
		for y := 8; y < 56; y++ {
			for x := 12; x < 52; x++ {
				pixel := img.RGBAAt(x, y)
				if pixel.A == 255 && pixel.R < bg.R/2 {
					ink++
				}
			}
		}
		if ink == 0 {
			t.Fatal("initials did not render over background")
		}
	}
	if len(colors) < 2 {
		t.Fatal("different identities all received the same color")
	}
	for _, tc := range [][2]string{{"FULL NAME", "0"}, {"M", "-1"}, {"M", "999"}, {"\n", "0"}} {
		if r.InitialsAvatar(tc[0], tc[1]) != nil {
			t.Errorf("accepted invalid avatar %q", tc)
		}
	}
}
