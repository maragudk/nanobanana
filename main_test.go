package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestValidateOutputPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"output.jpg", false},
		{"output.jpeg", false},
		{"output.JPG", false},
		{"dir.png/output.Jpeg", false},
		{"output.png", true},
		{"output.PNG", true},
		{"output.txt", true},
		{"output", true},
		{"output.jpg.png", true},
		{"jpg", true},
		{"dir.jpg/output", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := validateOutputPath(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("validateOutputPath(%q) = nil, expected an error", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateOutputPath(%q) = %v, expected nil", tt.path, err)
			}
		})
	}
}

func TestToJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for x := range 4 {
		for y := range 3 {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 20, A: 255})
		}
	}

	t.Run("should convert PNG data to JPEG with the same dimensions", func(t *testing.T) {
		var pngData bytes.Buffer
		if err := png.Encode(&pngData, img); err != nil {
			t.Fatal(err)
		}

		out, err := toJPEG(pngData.Bytes())
		if err != nil {
			t.Fatal(err)
		}

		cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		if format != "jpeg" {
			t.Errorf("format = %q, expected %q", format, "jpeg")
		}
		if cfg.Width != 4 || cfg.Height != 3 {
			t.Errorf("size = %dx%d, expected 4x3", cfg.Width, cfg.Height)
		}
	})

	t.Run("should composite transparent PNG pixels onto white", func(t *testing.T) {
		transparent := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		var pngData bytes.Buffer
		if err := png.Encode(&pngData, transparent); err != nil {
			t.Fatal(err)
		}

		out, err := toJPEG(pngData.Bytes())
		if err != nil {
			t.Fatal(err)
		}

		decoded, err := jpeg.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := decoded.At(4, 4).RGBA()
		if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
			t.Errorf("pixel = (%d, %d, %d), expected near white", r>>8, g>>8, b>>8)
		}
	})

	t.Run("should return JPEG data unchanged", func(t *testing.T) {
		var jpegData bytes.Buffer
		if err := jpeg.Encode(&jpegData, img, nil); err != nil {
			t.Fatal(err)
		}

		out, err := toJPEG(jpegData.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, jpegData.Bytes()) {
			t.Error("expected JPEG data to be returned unchanged")
		}
	})

	t.Run("should return an error for data that is not an image", func(t *testing.T) {
		if _, err := toJPEG([]byte("not an image")); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("should return an error for empty data", func(t *testing.T) {
		if _, err := toJPEG(nil); err == nil {
			t.Error("expected an error, got nil")
		}
	})
}
