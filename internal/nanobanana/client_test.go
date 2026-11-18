package nanobanana_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genai"

	"maragu.dev/nanobanana/internal/nanobanana"
)

func TestClient_Generate(t *testing.T) {
	t.Run("should send the input images, prompt, aspect ratio, and size, and return the final image", func(t *testing.T) {
		var body struct {
			Contents []struct {
				Role  string
				Parts []struct {
					Text       string
					InlineData *struct {
						MIMEType string `json:"mimeType"`
						Data     []byte
					} `json:"inlineData"`
				}
			}
			GenerationConfig struct {
				ImageConfig map[string]string `json:"imageConfig"`
			} `json:"generationConfig"`
		}
		var path string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			if err := json.Unmarshal(data, &body); err != nil {
				t.Error(err)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[
				{"text":"thinking","thought":true},
				{"inlineData":{"mimeType":"image/jpeg","data":%q},"thought":true},
				{"inlineData":{"mimeType":"image/jpeg","data":%q}}
			]}}]}`, base64.StdEncoding.EncodeToString([]byte("thought")), base64.StdEncoding.EncodeToString([]byte("final")))
		}))
		t.Cleanup(srv.Close)

		genaiClient, err := genai.NewClient(t.Context(), &genai.ClientConfig{
			APIKey:      "test",
			Backend:     genai.BackendGeminiAPI,
			HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL},
		})
		if err != nil {
			t.Fatal(err)
		}
		client := nanobanana.NewClient("test").WithGenAIClient(genaiClient)

		res, err := client.Generate(t.Context(), nanobanana.GenerateRequest{
			Prompt: "combine",
			InputImages: []nanobanana.Image{
				{Data: []byte("one"), MIMEType: "image/png"},
				{Data: []byte("two"), MIMEType: "image/webp"},
			},
			AspectRatio: "16:9",
			ImageSize:   "2K",
		})
		if err != nil {
			t.Fatal(err)
		}

		if string(res.Image) != "final" {
			t.Errorf("image = %q, expected %q", res.Image, "final")
		}
		if !strings.HasSuffix(path, "/models/gemini-nano-banana-2.1:generateContent") {
			t.Errorf("path = %v, expected the Nano Banana 2.1 generateContent endpoint", path)
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 3 {
			t.Fatalf("contents = %+v, expected one content with three parts", body.Contents)
		}
		parts := body.Contents[0].Parts
		if parts[0].InlineData == nil || string(parts[0].InlineData.Data) != "one" || parts[0].InlineData.MIMEType != "image/png" {
			t.Errorf("part 0 = %+v, expected the first image", parts[0])
		}
		if parts[1].InlineData == nil || string(parts[1].InlineData.Data) != "two" || parts[1].InlineData.MIMEType != "image/webp" {
			t.Errorf("part 1 = %+v, expected the second image", parts[1])
		}
		if parts[2].Text != "combine" {
			t.Errorf("part 2 = %+v, expected the prompt", parts[2])
		}
		if ic := body.GenerationConfig.ImageConfig; ic["aspectRatio"] != "16:9" || ic["imageSize"] != "2K" || len(ic) != 2 {
			t.Errorf("imageConfig = %v, expected only aspectRatio 16:9 and imageSize 2K", ic)
		}
	})
}
