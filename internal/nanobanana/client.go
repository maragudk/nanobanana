package nanobanana

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
	"maragu.dev/errors"
	google "maragu.dev/gai-google"
)

// model is the Nano Banana 2.1 model ID, used for all image generation.
const model = "gemini-nano-banana-2.1"

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

// Image is an input image with its MIME type, such as "image/png".
type Image struct {
	Data     []byte
	MIMEType string
}

// GenerateRequest to generate an image.
// AspectRatio (such as "16:9") and ImageSize ("1K", "2K", or "4K", uppercase only) are optional.
// When they are empty, the model decides. Up to 14 input images are supported.
type GenerateRequest struct {
	Prompt      string
	InputImages []Image
	AspectRatio string
	ImageSize   string
}

// GenerateResponse from image generation. Image is encoded image data, normally JPEG.
type GenerateResponse struct {
	Image []byte
}

// Generate an image from a prompt, optionally based on input images.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	res, err := c.genaiClient.Models.GenerateContent(ctx, model, buildContents(req), buildConfig(req))
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate content")
	}

	image, err := finalImage(res)
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{Image: image}, nil
}

// buildContents for a single user turn: each input image as its own inline part, in order, followed by the prompt.
func buildContents(req GenerateRequest) []*genai.Content {
	parts := make([]*genai.Part, 0, len(req.InputImages)+1)
	for _, image := range req.InputImages {
		parts = append(parts, genai.NewPartFromBytes(image.Data, image.MIMEType))
	}
	parts = append(parts, genai.NewPartFromText(req.Prompt))

	return []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}
}

// buildConfig with an image config only if the aspect ratio or image size is set, so the API defaults apply otherwise.
// The output MIME type is not set, because [genai.ImageConfig] doesn't support it in the Gemini API.
func buildConfig(req GenerateRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}
	if req.AspectRatio != "" || req.ImageSize != "" {
		config.ImageConfig = &genai.ImageConfig{
			AspectRatio: req.AspectRatio,
			ImageSize:   req.ImageSize,
		}
	}
	return config
}

// finalImage returns the last non-thought image in the first candidate of res.
// Image models think before they answer, and when thoughts are included in the response,
// interim thought images come before the final one.
// When there is no image, the error includes the reasons the API gives.
func finalImage(res *genai.GenerateContentResponse) ([]byte, error) {
	if len(res.Candidates) == 0 {
		if res.PromptFeedback != nil && res.PromptFeedback.BlockReason != "" {
			return nil, errors.Newf("prompt blocked: %v", strings.TrimSpace(fmt.Sprint(res.PromptFeedback.BlockReason, " ", res.PromptFeedback.BlockReasonMessage)))
		}
		return nil, errors.New("no candidates in response")
	}

	candidate := res.Candidates[0]
	if candidate.Content == nil {
		return nil, errors.Newf("no content in candidate%v", finishDetails(candidate))
	}

	var image []byte
	var texts []string
	for _, part := range candidate.Content.Parts {
		if part.Thought {
			continue
		}
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			image = part.InlineData.Data
		}
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}

	if image == nil {
		if len(texts) > 0 {
			return nil, errors.Newf("no image data in response%v, the model said: %v", finishDetails(candidate), strings.Join(texts, " "))
		}
		return nil, errors.Newf("no image data in response%v", finishDetails(candidate))
	}

	return image, nil
}

// finishDetails of candidate for an error message, such as " (finish reason IMAGE_SAFETY: ...)", or an empty string.
func finishDetails(candidate *genai.Candidate) string {
	if candidate.FinishReason == "" {
		return ""
	}
	if candidate.FinishMessage == "" {
		return fmt.Sprintf(" (finish reason %v)", candidate.FinishReason)
	}
	return fmt.Sprintf(" (finish reason %v: %v)", candidate.FinishReason, candidate.FinishMessage)
}
