// Package ogcard draws the picture Viber, Facebook, Messenger, X and Google
// show for a shared movie or series link (my-app's recordMetadata points
// og:image here): the backdrop, the poster, the title and the БГ субтитри
// badge — Bulgarian only, like everything crawlers read on the site.
//
// It lives here and not in the website's Worker because a Worker on the
// free plan gets 10ms of CPU per request, far too little to draw a picture;
// this gateway is kept awake by heartbeat.StartKeepAlive, so crawlers never
// wait out a cold start. JPEG rather than PNG: a tenth of the size, which
// WhatsApp and Viber need to show a preview at all.
package ogcard

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const (
	Width  = 1200
	Height = 630

	padding    = 56
	posterW    = 340
	posterH    = 510
	columnGap  = 52
	textLeft   = padding + posterW + columnGap
	textWidth  = Width - padding - textLeft
	siteName   = "AuroraFlix"
	callToPlay = "Гледай онлайн безплатно"
)

var (
	//go:embed fonts/Manrope-600.ttf
	semiBoldTTF []byte
	//go:embed fonts/Manrope-800.ttf
	extraBoldTTF []byte

	semiBold  = mustParse(semiBoldTTF)
	extraBold = mustParse(extraBoldTTF)

	background = color.RGBA{7, 9, 15, 255}
	accent     = color.RGBA{52, 211, 153, 255}
	amber      = color.RGBA{251, 191, 36, 255}
	white      = color.RGBA{255, 255, 255, 255}
)

func mustParse(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	return f
}

func face(f *opentype.Font, size float64) font.Face {
	ff, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return ff
}

// Card is what goes on the picture; the images are already decoded (nil
// when the record has none).
type Card struct {
	Title    string
	Subtitle string // "Clayface · 2026 · Филм"
	Rating   float64
	Genres   []string
	Badges   []string
	Poster   image.Image
	Backdrop image.Image
}

// Render draws the card as a JPEG.
func Render(card Card) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)

	if card.Backdrop != nil {
		drawSoftBackdrop(canvas, card.Backdrop)
	}
	darkenTowardsText(canvas)

	if card.Poster != nil {
		drawPoster(canvas, card.Poster)
	}

	left := textLeft
	if card.Poster == nil {
		left = padding
	}
	width := Width - padding - left
	drawText(canvas, card, left, width)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 84}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// drawSoftBackdrop covers the canvas with the backdrop, blurred by shrinking
// it to a few dozen pixels and stretching it back: soft behind the text, and
// smooth areas compress far better than detail.
func drawSoftBackdrop(canvas *image.RGBA, src image.Image) {
	b := src.Bounds()
	// Crop to the card's aspect ratio (object-fit: cover).
	scale := math.Max(float64(Width)/float64(b.Dx()), float64(Height)/float64(b.Dy()))
	cropW := int(float64(Width) / scale)
	cropH := int(float64(Height) / scale)
	x0 := b.Min.X + (b.Dx()-cropW)/2
	y0 := b.Min.Y + (b.Dy()-cropH)/2
	crop := image.Rect(x0, y0, x0+cropW, y0+cropH)

	small := image.NewRGBA(image.Rect(0, 0, 96, 50))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), src, crop, draw.Src, nil)
	draw.BiLinear.Scale(canvas, canvas.Bounds(), small, small.Bounds(), draw.Src, nil)
}

// darkenTowardsText lays the site's dark background over the backdrop: all
// but opaque on the left, where the text goes, lighter to the right.
func darkenTowardsText(canvas *image.RGBA) {
	for x := 0; x < Width; x++ {
		t := float64(x) / float64(Width-1)
		alpha := 0.97 - 0.07*math.Min(t/0.45, 1)
		if t > 0.45 {
			alpha = 0.9 - 0.35*(t-0.45)/0.55
		}
		for y := 0; y < Height; y++ {
			i := canvas.PixOffset(x, y)
			for c, bg := range [3]uint8{background.R, background.G, background.B} {
				canvas.Pix[i+c] = uint8(float64(canvas.Pix[i+c])*(1-alpha) + float64(bg)*alpha + 0.5)
			}
		}
	}
}

func drawPoster(canvas *image.RGBA, poster image.Image) {
	top := (Height - posterH) / 2
	rect := image.Rect(padding, top, padding+posterW, top+posterH)

	// A faint frame, then the picture inside it, both with rounded corners.
	fillRoundedRect(canvas, rect.Inset(-2), 26, color.NRGBA{255, 255, 255, 30})

	scaled := image.NewRGBA(image.Rect(0, 0, posterW, posterH))
	b := poster.Bounds()
	scale := math.Max(float64(posterW)/float64(b.Dx()), float64(posterH)/float64(b.Dy()))
	cropW := int(float64(posterW) / scale)
	cropH := int(float64(posterH) / scale)
	x0 := b.Min.X + (b.Dx()-cropW)/2
	y0 := b.Min.Y + (b.Dy()-cropH)/2
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), poster, image.Rect(x0, y0, x0+cropW, y0+cropH), draw.Src, nil)

	mask := roundedMask(posterW, posterH, 24)
	draw.DrawMask(canvas, rect, scaled, image.Point{}, mask, image.Point{}, draw.Over)
}

func drawText(canvas *image.RGBA, card Card, left, width int) {
	// The site's name, top.
	top := (Height - posterH) / 2
	fillCircle(canvas, float32(left+8), float32(top+22), 8, accent)
	brand := face(extraBold, 30)
	drawString(canvas, brand, siteName, left+28, top+33, white)

	// Title, subtitle and rating/genres, in the middle.
	titleSize := 78.0
	switch n := len([]rune(card.Title)); {
	case n > 40:
		titleSize = 52
	case n > 22:
		titleSize = 64
	}
	titleFace := face(extraBold, titleSize)
	lines := wrap(titleFace, card.Title, width, 2)
	lineHeight := int(titleSize * 1.08)

	small := face(semiBold, 30)
	meta := face(semiBold, 28)
	hasMeta := card.Rating > 0 || len(card.Genres) > 0

	block := len(lines) * lineHeight
	if card.Subtitle != "" {
		block += 18 + 34
	}
	if hasMeta {
		block += 18 + 32
	}
	y := Height/2 - block/2

	for _, line := range lines {
		y += lineHeight
		drawString(canvas, titleFace, line, left, y-int(titleSize*0.2), white)
	}
	if card.Subtitle != "" {
		y += 18 + 34
		drawString(canvas, small, fit(small, card.Subtitle, width), left, y-6, color.NRGBA{255, 255, 255, 180})
	}
	if hasMeta {
		y += 18 + 32
		x := left
		if card.Rating > 0 {
			fillStar(canvas, float32(x+14), float32(y-15), 14, amber)
			x += 38
			rating := strconv.FormatFloat(card.Rating, 'f', 1, 64)
			x = drawString(canvas, face(extraBold, 28), rating, x, y-4, amber) + 22
		}
		if len(card.Genres) > 0 {
			drawString(canvas, meta, fit(meta, strings.Join(card.Genres, " · "), left+width-x), x, y-4, color.NRGBA{255, 255, 255, 190})
		}
	}

	// Badges, and the call to watch, bottom.
	bottom := top + posterH
	badgeFace := face(extraBold, 26)
	x := left
	for i, label := range card.Badges {
		w := measure(badgeFace, label) + 44
		rect := image.Rect(x, bottom-56, x+w, bottom)
		var ink color.Color = white
		if i == 0 {
			fillRoundedRect(canvas, rect, 28, accent)
			ink = color.RGBA{4, 19, 13, 255}
		} else {
			fillRoundedRect(canvas, rect, 28, color.NRGBA{255, 255, 255, 64})
			fillRoundedRect(canvas, rect.Inset(2), 26, color.RGBA{30, 32, 38, 255})
		}
		drawString(canvas, badgeFace, label, x+22, bottom-19, ink)
		x += w + 14
	}
	cta := face(semiBold, 26)
	if ctaW := measure(cta, callToPlay); x+ctaW+24 <= left+width {
		drawString(canvas, cta, callToPlay, left+width-ctaW, bottom-19, color.NRGBA{255, 255, 255, 155})
	}
}

// drawString draws s with its baseline at y and returns where it ends.
func drawString(canvas *image.RGBA, f font.Face, s string, x, y int, ink color.Color) int {
	d := &font.Drawer{Dst: canvas, Src: image.NewUniform(ink), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
	return d.Dot.X.Round()
}

func measure(f font.Face, s string) int {
	return font.MeasureString(f, s).Round()
}

// fit cuts s down to width with an ellipsis.
func fit(f font.Face, s string, width int) string {
	if measure(f, s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && measure(f, strings.TrimSpace(string(runes))+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimSpace(string(runes)) + "…"
}

// wrap breaks s into at most maxLines lines of width, the last one cut short.
func wrap(f font.Face, s string, width, maxLines int) []string {
	var lines []string
	line := ""
	words := strings.Fields(s)
	for i, word := range words {
		next := strings.TrimSpace(line + " " + word)
		if line == "" || measure(f, next) <= width {
			line = next
			continue
		}
		if len(lines) == maxLines-1 {
			return append(lines, fit(f, strings.Join(append([]string{line}, words[i:]...), " "), width))
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, fit(f, line, width))
}

// --- shapes, anti-aliased ---

func roundedPath(r *vector.Rasterizer, x0, y0, x1, y1, radius float32) {
	const k = 0.5523 // control point distance for a quarter circle
	r.MoveTo(x0+radius, y0)
	r.LineTo(x1-radius, y0)
	r.CubeTo(x1-radius+radius*k, y0, x1, y0+radius-radius*k, x1, y0+radius)
	r.LineTo(x1, y1-radius)
	r.CubeTo(x1, y1-radius+radius*k, x1-radius+radius*k, y1, x1-radius, y1)
	r.LineTo(x0+radius, y1)
	r.CubeTo(x0+radius-radius*k, y1, x0, y1-radius+radius*k, x0, y1-radius)
	r.LineTo(x0, y0+radius)
	r.CubeTo(x0, y0+radius-radius*k, x0+radius-radius*k, y0, x0+radius, y0)
	r.ClosePath()
}

func roundedMask(w, h int, radius float32) *image.Alpha {
	r := vector.NewRasterizer(w, h)
	roundedPath(r, 0, 0, float32(w), float32(h), radius)
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	r.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	return mask
}

func fillRoundedRect(canvas *image.RGBA, rect image.Rectangle, radius float32, fill color.Color) {
	radius = float32(math.Min(float64(radius), float64(rect.Dy())/2))
	mask := roundedMask(rect.Dx(), rect.Dy(), radius)
	draw.DrawMask(canvas, rect, image.NewUniform(fill), image.Point{}, mask, image.Point{}, draw.Over)
}

func fillCircle(canvas *image.RGBA, cx, cy, radius float32, fill color.Color) {
	d := int(radius * 2)
	rect := image.Rect(int(cx-radius), int(cy-radius), int(cx-radius)+d, int(cy-radius)+d)
	fillRoundedRect(canvas, rect, radius, fill)
}

func fillStar(canvas *image.RGBA, cx, cy, radius float32, fill color.Color) {
	r := vector.NewRasterizer(Width, Height)
	for i := 0; i < 10; i++ {
		rr := radius
		if i%2 == 1 {
			rr = radius * 0.45
		}
		angle := -math.Pi/2 + float64(i)*math.Pi/5
		x := cx + rr*float32(math.Cos(angle))
		y := cy + rr*float32(math.Sin(angle))
		if i == 0 {
			r.MoveTo(x, y)
		} else {
			r.LineTo(x, y)
		}
	}
	r.ClosePath()
	r.Draw(canvas, canvas.Bounds(), image.NewUniform(fill), image.Point{})
}
