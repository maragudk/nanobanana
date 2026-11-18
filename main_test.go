package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/genai"
	"maragu.dev/clir"

	"maragu.dev/nanobanana/internal/nanobanana"
)

func TestParseGenerateArgs(t *testing.T) {
	t.Run("should parse output path, prompt, and output format with no flags", func(t *testing.T) {
		opts, err := parseGenerateArgs([]string{"out.jpg", "a sunset"})
		if err != nil {
			t.Fatal(err)
		}
		if opts.outputPath != "out.jpg" || opts.prompt != "a sunset" || opts.format != "jpeg" {
			t.Errorf("opts = %+v, expected out.jpg, a sunset, jpeg", opts)
		}
		if opts.aspectRatio != "" || opts.imageSize != "" || len(opts.inputPaths) != 0 {
			t.Errorf("opts = %+v, expected no aspect ratio, image size, or input paths", opts)
		}
	})

	t.Run("should collect repeated -i flags in order", func(t *testing.T) {
		opts, err := parseGenerateArgs([]string{"-i", "a.png", "-i", "b.jpg", "out.png", "combine"})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(opts.inputPaths, []string{"a.png", "b.jpg"}) {
			t.Errorf("inputPaths = %v, expected [a.png b.jpg]", opts.inputPaths)
		}
	})

	t.Run("should require an output path and a prompt", func(t *testing.T) {
		_, err := parseGenerateArgs([]string{"out.jpg"})
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("err = %v, expected a usage error", err)
		}
	})

	t.Run("should reject flags after the prompt instead of ignoring them", func(t *testing.T) {
		_, err := parseGenerateArgs([]string{"out.jpg", "a sunset", "-size", "4K"})
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("err = %v, expected a usage error", err)
		}
	})

	t.Run("should reject an unquoted prompt split into several arguments", func(t *testing.T) {
		_, err := parseGenerateArgs([]string{"out.jpg", "a", "sunset"})
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("err = %v, expected a usage error", err)
		}
	})

	t.Run("should accept up to 14 input images and reject more", func(t *testing.T) {
		var args []string
		for i := range 14 {
			args = append(args, "-i", fmt.Sprint(i, ".png"))
		}

		if _, err := parseGenerateArgs(append(slices.Clone(args), "out.jpg", "p")); err != nil {
			t.Errorf("expected 14 input images to be accepted, got %v", err)
		}

		_, err := parseGenerateArgs(append(slices.Clone(args), "-i", "14.png", "out.jpg", "p"))
		if err == nil || !strings.Contains(err.Error(), "too many input images (15), the maximum is 14") {
			t.Errorf("err = %v, expected an error mentioning the limit of 14", err)
		}
	})

	aspectTests := []struct {
		value   string
		wantErr bool
	}{
		{"1:1", false},
		{"1:4", false},
		{"1:8", false},
		{"2:3", false},
		{"3:2", false},
		{"3:4", false},
		{"4:1", false},
		{"4:3", false},
		{"4:5", false},
		{"5:4", false},
		{"8:1", false},
		{"9:16", false},
		{"16:9", false},
		{"21:9", false},
		{"2:1", true},
		{"16:10", true},
		{"1x1", true},
		{"square", true},
		{" 1:1", true},
	}
	for _, tt := range aspectTests {
		t.Run(fmt.Sprintf("should %v aspect ratio %q", acceptOrReject(tt.wantErr), tt.value), func(t *testing.T) {
			opts, err := parseGenerateArgs([]string{"-aspect", tt.value, "out.jpg", "p"})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for aspect ratio %q", tt.value)
				}
				if !strings.Contains(err.Error(), "1:1, 1:4, 1:8, 2:3, 3:2, 3:4, 4:1, 4:3, 4:5, 5:4, 8:1, 9:16, 16:9, 21:9") {
					t.Errorf("err = %v, expected it to list the allowed aspect ratios", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts.aspectRatio != tt.value {
				t.Errorf("aspectRatio = %q, expected %q", opts.aspectRatio, tt.value)
			}
		})
	}

	sizeTests := []struct {
		value    string
		expected string
		wantErr  bool
	}{
		{"1K", "1K", false},
		{"2K", "2K", false},
		{"4K", "4K", false},
		{"1k", "1K", false},
		{"2k", "2K", false},
		{"4k", "4K", false},
		{"512", "", true},
		{"8K", "", true},
		{"1024", "", true},
		{"big", "", true},
	}
	for _, tt := range sizeTests {
		t.Run(fmt.Sprintf("should %v image size %q", acceptOrReject(tt.wantErr), tt.value), func(t *testing.T) {
			opts, err := parseGenerateArgs([]string{"-size", tt.value, "out.jpg", "p"})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for image size %q", tt.value)
				}
				if !strings.Contains(err.Error(), "1K, 2K, 4K") {
					t.Errorf("err = %v, expected it to list the allowed sizes", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts.imageSize != tt.expected {
				t.Errorf("imageSize = %q, expected %q", opts.imageSize, tt.expected)
			}
		})
	}

	extensionTests := []struct {
		path     string
		expected string
		wantErr  bool
	}{
		{"output.jpg", "jpeg", false},
		{"output.jpeg", "jpeg", false},
		{"output.JPG", "jpeg", false},
		{"dir.png/output.Jpeg", "jpeg", false},
		{"output.png", "png", false},
		{"output.PNG", "png", false},
		{"dir.jpg/output.Png", "png", false},
		{"output.jpg.png", "png", false},
		{"output.txt", "", true},
		{"output.webp", "", true},
		{"output", "", true},
		{"jpg", "", true},
		{"dir.jpg/output", "", true},
	}
	for _, tt := range extensionTests {
		t.Run(fmt.Sprintf("should %v output path %q", acceptOrReject(tt.wantErr), tt.path), func(t *testing.T) {
			opts, err := parseGenerateArgs([]string{tt.path, "p"})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for output path %q", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts.format != tt.expected {
				t.Errorf("format = %q, expected %q", opts.format, tt.expected)
			}
		})
	}
}

func TestDetectImageMIMEType(t *testing.T) {
	img := testImage()

	var pngData, jpegData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		data     []byte
		expected string
		wantErr  bool
	}{
		{name: "should detect PNG", data: pngData.Bytes(), expected: "image/png"},
		{name: "should detect JPEG", data: jpegData.Bytes(), expected: "image/jpeg"},
		{name: "should detect WebP", data: []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), expected: "image/webp"},
		{name: "should detect HEIC", data: []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), expected: "image/heic"},
		{name: "should detect HEIF", data: []byte("\x00\x00\x00\x18ftypmif1\x00\x00\x00\x00mif1heic"), expected: "image/heif"},
		{name: "should reject AVIF with the HEIF major brand", data: []byte("\x00\x00\x00\x1cftypmif1\x00\x00\x00\x00mif1avifmiaf"), wantErr: true},
		{name: "should reject a HEIF image sequence", data: []byte("\x00\x00\x00\x18ftypmsf1\x00\x00\x00\x00msf1hevc"), wantErr: true},
		{name: "should reject GIF, which the API does not accept", data: []byte("GIF89a\x01\x00\x01\x00"), wantErr: true},
		{name: "should reject an MP4 video", data: []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), wantErr: true},
		{name: "should reject text", data: []byte("not an image"), wantErr: true},
		{name: "should reject a single byte without panicking", data: []byte{0xFF}, wantErr: true},
		{name: "should reject empty data", data: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, err := detectImageMIMEType(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("detectImageMIMEType = %q, expected an error", mimeType)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mimeType != tt.expected {
				t.Errorf("detectImageMIMEType = %q, expected %q", mimeType, tt.expected)
			}
		})
	}
}

func TestConvertImage(t *testing.T) {
	img := testImage()

	var pngData, jpegData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}

	t.Run("should convert PNG data to JPEG with the same dimensions", func(t *testing.T) {
		out, err := convertImage(pngData.Bytes(), "jpeg")
		if err != nil {
			t.Fatal(err)
		}
		assertImage(t, out, "jpeg", 4, 3)
	})

	t.Run("should convert JPEG data to PNG with the same dimensions", func(t *testing.T) {
		out, err := convertImage(jpegData.Bytes(), "png")
		if err != nil {
			t.Fatal(err)
		}
		assertImage(t, out, "png", 4, 3)

		// The bit depth is the first byte after the width and height in the IHDR chunk.
		if bitDepth := out[24]; bitDepth != 8 {
			t.Errorf("bit depth = %d, expected 8, because JPEG has 8 bits per channel", bitDepth)
		}
	})

	t.Run("should composite transparent PNG pixels onto white when converting to JPEG", func(t *testing.T) {
		transparent := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		var data bytes.Buffer
		if err := png.Encode(&data, transparent); err != nil {
			t.Fatal(err)
		}

		out, err := convertImage(data.Bytes(), "jpeg")
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

	t.Run("should return JPEG data unchanged when converting to JPEG", func(t *testing.T) {
		out, err := convertImage(jpegData.Bytes(), "jpeg")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, jpegData.Bytes()) {
			t.Error("expected JPEG data to be returned unchanged")
		}
	})

	t.Run("should return PNG data unchanged when converting to PNG", func(t *testing.T) {
		out, err := convertImage(pngData.Bytes(), "png")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, pngData.Bytes()) {
			t.Error("expected PNG data to be returned unchanged")
		}
	})

	t.Run("should return an error for data that is not an image", func(t *testing.T) {
		for _, format := range []string{"jpeg", "png"} {
			if _, err := convertImage([]byte("not an image"), format); err == nil {
				t.Errorf("expected an error converting to %v, got nil", format)
			}
		}
	})

	t.Run("should return an error for empty data", func(t *testing.T) {
		for _, format := range []string{"jpeg", "png"} {
			if _, err := convertImage(nil, format); err == nil {
				t.Errorf("expected an error converting to %v, got nil", format)
			}
		}
	})
}

// testImage returns a small opaque image.
func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for x := range 4 {
		for y := range 3 {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 20, A: 255})
		}
	}
	return img
}

// assertImage decodes data and checks its format and dimensions.
func assertImage(t *testing.T, data []byte, format string, width, height int) {
	t.Helper()

	cfg, gotFormat, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if gotFormat != format {
		t.Errorf("format = %q, expected %q", gotFormat, format)
	}
	if cfg.Width != width || cfg.Height != height {
		t.Errorf("size = %dx%d, expected %dx%d", cfg.Width, cfg.Height, width, height)
	}
}

func TestReadInputImage(t *testing.T) {
	t.Run("should read an image and detect its MIME type", func(t *testing.T) {
		var data bytes.Buffer
		if err := png.Encode(&data, testImage()); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "input.jpg")
		if err := os.WriteFile(path, data.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}

		img, err := readInputImage(path)
		if err != nil {
			t.Fatal(err)
		}
		if img.MIMEType != "image/png" {
			t.Errorf("MIMEType = %q, expected %q from the contents, not the extension", img.MIMEType, "image/png")
		}
		if !bytes.Equal(img.Data, data.Bytes()) {
			t.Error("expected the file contents as data")
		}
	})

	t.Run("should return an error naming the file if it isn't an image", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "notes.png")
		if err := os.WriteFile(path, []byte("not an image"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := readInputImage(path)
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("err = %v, expected an error naming %v", err, path)
		}
	})

	t.Run("should return an error naming the file if it doesn't exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.png")

		_, err := readInputImage(path)
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("err = %v, expected an error naming %v", err, path)
		}
	})
}

func TestGenerateHandler(t *testing.T) {
	t.Run("should write the final image, not the thought image, converted to the output format", func(t *testing.T) {
		thought := encodeJPEG(t, image.NewRGBA(image.Rect(0, 0, 2, 2)))
		final := encodeJPEG(t, testImage())
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[
				{"inlineData":{"mimeType":"image/jpeg","data":%q},"thought":true},
				{"inlineData":{"mimeType":"image/jpeg","data":%q}}
			]}}]}`, base64.StdEncoding.EncodeToString(thought), base64.StdEncoding.EncodeToString(final))
		})
		outputPath := filepath.Join(t.TempDir(), "out.png")

		err := generateHandler(client)(clir.Context{Args: []string{outputPath, "p"}, Ctx: t.Context(), Out: io.Discard, Err: io.Discard})
		if err != nil {
			t.Fatal(err)
		}

		out, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		assertImage(t, out, "png", 4, 3)
	})

	tests := []struct {
		name string
		args func(t *testing.T, dir string) []string
	}{
		{
			name: "should not call the API if an input file isn't an image",
			args: func(t *testing.T, dir string) []string {
				path := filepath.Join(dir, "notes.png")
				if err := os.WriteFile(path, []byte("not an image"), 0644); err != nil {
					t.Fatal(err)
				}
				return []string{"-i", path, filepath.Join(dir, "out.jpg"), "p"}
			},
		},
		{
			name: "should not call the API if the output directory doesn't exist",
			args: func(t *testing.T, dir string) []string {
				return []string{filepath.Join(dir, "missing", "out.jpg"), "p"}
			},
		},
		{
			name: "should not call the API if the output path's parent is a file",
			args: func(t *testing.T, dir string) []string {
				path := filepath.Join(dir, "file")
				if err := os.WriteFile(path, nil, 0644); err != nil {
					t.Fatal(err)
				}
				return []string{filepath.Join(path, "out.jpg"), "p"}
			},
		},
		{
			name: "should not call the API if the aspect ratio is invalid",
			args: func(t *testing.T, dir string) []string {
				return []string{"-aspect", "7:3", filepath.Join(dir, "out.jpg"), "p"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected API request to %v", r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			})

			err := generateHandler(client)(clir.Context{Args: tt.args(t, t.TempDir()), Ctx: t.Context(), Out: io.Discard, Err: io.Discard})
			if err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// newTestClient returns a [nanobanana.Client] that sends its requests to a test server with the given handler.
func newTestClient(t *testing.T, h http.HandlerFunc) *nanobanana.Client {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	genaiClient, err := genai.NewClient(t.Context(), &genai.ClientConfig{
		APIKey:      "test",
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	return nanobanana.NewClient("test").WithGenAIClient(genaiClient)
}

// encodeJPEG returns img encoded as JPEG.
func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// acceptOrReject for subtest names.
func acceptOrReject(wantErr bool) string {
	if wantErr {
		return "reject"
	}
	return "accept"
}
