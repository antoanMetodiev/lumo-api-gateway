package ogcard

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestRenderDrawsAJpegCard(t *testing.T) {
	poster := image.NewRGBA(image.Rect(0, 0, 342, 513))
	for i := range poster.Pix {
		poster.Pix[i] = 200
	}
	out, err := Render(Card{
		Title:    "Клейфейс: едно доста дълго заглавие, което трябва да се пренесе на втори ред",
		Subtitle: "Clayface · 2026 · Филм",
		Rating:   8.1,
		Genres:   []string{"Ужаси", "Трилър"},
		Badges:   []string{"БГ субтитри", "БГ аудио"},
		Poster:   poster,
		Backdrop: image.NewUniform(color.RGBA{120, 20, 20, 255}),
	})
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != Width || b.Dy() != Height {
		t.Fatalf("got %v, want %dx%d", b, Width, Height)
	}
	if len(out) > 300<<10 {
		t.Fatalf("card is %d KB, too big for messenger previews", len(out)>>10)
	}
}

func TestRenderWithoutImages(t *testing.T) {
	if _, err := Render(Card{Title: "Без картинки", Badges: []string{"БГ субтитри"}}); err != nil {
		t.Fatal(err)
	}
}

func TestWrapKeepsToTwoLines(t *testing.T) {
	f := face(extraBold, 64)
	lines := wrap(f, "Едно много много много дълго заглавие на филм, което никога не свършва и продължава", 696, 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines: %q", len(lines), lines)
	}
	for _, line := range lines {
		if measure(f, line) > 696 {
			t.Fatalf("line too wide: %q", line)
		}
	}
}
