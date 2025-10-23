package nanobanana

import (
	"context"
	"io"

	"google.golang.org/genai"
	"maragu.dev/errors"
	google "maragu.dev/gai-google"
)

const (
	imageModel = "gemini-2.5-flash-image"
)

// Client is a Nano Banana API client.
type Client struct {
	genaiClient *genai.Client
}

// NewClient creates a new Nano Banana API client.
func NewClient(apiKey string) *Client {
	gaiClient := google.NewClient(google.NewClientOptions{
		Key: apiKey,
	})
	return &Client{
		genaiClient: gaiClient.Client,
	}
}

// WithGenAIClient sets a custom genai client (for testing).
func (c *Client) WithGenAIClient(client *genai.Client) *Client {
	c.genaiClient = client
	return c
}

// GenerateRequest represents a request to generate an image.
type GenerateRequest struct {
	Prompt         string
	InputImage     io.Reader
	Count          int
	OutputMIMEType string
}

// GenerateResponse represents the response from image generation.
type GenerateResponse struct {
	Images [][]byte
}

// Generate generates an image from a prompt, optionally editing an existing image.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Build the content parts
	var contents []*genai.Content

	// If there's an input image, include it in the prompt
	if req.InputImage != nil {
		imageData, err := io.ReadAll(req.InputImage)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read input image")
		}

		// Determine MIME type from the image data or default to PNG
		mimeType := "image/png"
		if len(imageData) > 0 && imageData[0] == 0xFF && imageData[1] == 0xD8 {
			mimeType = "image/jpeg"
		}

		contents = []*genai.Content{
			{
				Parts: []*genai.Part{
					genai.NewPartFromBytes(imageData, mimeType),
					genai.NewPartFromText(req.Prompt),
				},
				Role: "user",
			},
		}
	} else {
		contents = genai.Text(req.Prompt)
	}

	// Configure the request
	config := &genai.GenerateContentConfig{}
	// Note: Output image format is not configurable in GenerateContent API
	// Images are returned in a default format

	// Generate images (Gemini 2.5 Flash Image generates one at a time)
	var images [][]byte
	for i := 0; i < req.Count; i++ {
		resp, err := c.genaiClient.Models.GenerateContent(ctx, imageModel, contents, config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate content")
		}

		// Extract image from response
		if len(resp.Candidates) == 0 {
			return nil, errors.New("no candidates in response")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil {
			return nil, errors.New("no content in candidate")
		}

		// Find the image part
		var imageData []byte
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != nil {
				imageData = part.InlineData.Data
				break
			}
		}

		if imageData == nil {
			return nil, errors.New("no image data in response")
		}

		images = append(images, imageData)
	}

	return &GenerateResponse{Images: images}, nil
}

