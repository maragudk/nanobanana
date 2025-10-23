package nanobanana

import (
	"context"
	"io"

	"google.golang.org/genai"
	"maragu.dev/errors"
	google "maragu.dev/gai-google"
)

const (
	imageModel = "imagen-4.0-generate-001"
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
	// If there's an input image, use EditImage
	if req.InputImage != nil {
		return c.editImage(ctx, req)
	}

	// Otherwise, generate from scratch
	config := &genai.GenerateImagesConfig{
		NumberOfImages: int32(req.Count),
	}

	// Set output MIME type if specified
	if req.OutputMIMEType != "" {
		config.OutputMIMEType = req.OutputMIMEType
	}

	resp, err := c.genaiClient.Models.GenerateImages(ctx, imageModel, req.Prompt, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate images")
	}

	// Convert generated images to byte slices
	images := make([][]byte, len(resp.GeneratedImages))
	for i, img := range resp.GeneratedImages {
		if img.Image == nil {
			return nil, errors.New("image data is nil")
		}

		images[i] = img.Image.ImageBytes
	}

	return &GenerateResponse{Images: images}, nil
}

func (c *Client) editImage(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Read the input image data
	imageData, err := io.ReadAll(req.InputImage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read input image")
	}

	// Create image from bytes
	img := &genai.Image{
		ImageBytes: imageData,
	}

	// Use EditImage API
	config := &genai.EditImageConfig{
		NumberOfImages: int32(req.Count),
	}

	// Set output MIME type if specified
	if req.OutputMIMEType != "" {
		config.OutputMIMEType = req.OutputMIMEType
	}

	resp, err := c.genaiClient.Models.EditImage(ctx, imageModel, req.Prompt, []genai.ReferenceImage{genai.NewRawReferenceImage(img, 1)}, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to edit image")
	}

	// Convert edited images to byte slices
	images := make([][]byte, len(resp.GeneratedImages))
	for i, img := range resp.GeneratedImages {
		if img.Image == nil {
			return nil, errors.New("image data is nil")
		}

		images[i] = img.Image.ImageBytes
	}

	return &GenerateResponse{Images: images}, nil
}
