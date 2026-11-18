package nanobanana

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestBuildContents(t *testing.T) {
	t.Run("should send only the prompt when there are no input images", func(t *testing.T) {
		contents := buildContents(GenerateRequest{Prompt: "a sunset"})

		if len(contents) != 1 || len(contents[0].Parts) != 1 {
			t.Fatalf("contents = %+v, expected one content with one part", contents)
		}
		if contents[0].Parts[0].Text != "a sunset" {
			t.Errorf("text = %q, expected %q", contents[0].Parts[0].Text, "a sunset")
		}
		if contents[0].Role != genai.RoleUser {
			t.Errorf("role = %q, expected %q", contents[0].Role, genai.RoleUser)
		}
	})

	t.Run("should send each input image as its own inline part, in order, followed by the prompt", func(t *testing.T) {
		contents := buildContents(GenerateRequest{
			Prompt: "combine these",
			InputImages: []Image{
				{Data: []byte("first"), MIMEType: "image/png"},
				{Data: []byte("second"), MIMEType: "image/jpeg"},
			},
		})

		if len(contents) != 1 {
			t.Fatalf("got %d contents, expected 1", len(contents))
		}
		parts := contents[0].Parts
		if len(parts) != 3 {
			t.Fatalf("got %d parts, expected 3", len(parts))
		}

		for i, expected := range []Image{{Data: []byte("first"), MIMEType: "image/png"}, {Data: []byte("second"), MIMEType: "image/jpeg"}} {
			if parts[i].InlineData == nil {
				t.Fatalf("part %d has no inline data", i)
			}
			if !bytes.Equal(parts[i].InlineData.Data, expected.Data) || parts[i].InlineData.MIMEType != expected.MIMEType {
				t.Errorf("part %d = %q (%v), expected %q (%v)", i, parts[i].InlineData.Data, parts[i].InlineData.MIMEType, expected.Data, expected.MIMEType)
			}
		}
		if parts[2].Text != "combine these" {
			t.Errorf("last part text = %q, expected the prompt", parts[2].Text)
		}
	})
}

func TestBuildConfig(t *testing.T) {
	t.Run("should send no image config when neither aspect ratio nor size is set", func(t *testing.T) {
		config := buildConfig(GenerateRequest{})
		if config.ImageConfig != nil {
			t.Errorf("ImageConfig = %+v, expected nil", config.ImageConfig)
		}
	})

	t.Run("should send the aspect ratio and size when set", func(t *testing.T) {
		config := buildConfig(GenerateRequest{AspectRatio: "9:16", ImageSize: "2K"})
		if config.ImageConfig == nil {
			t.Fatal("ImageConfig is nil")
		}
		if config.ImageConfig.AspectRatio != "9:16" || config.ImageConfig.ImageSize != "2K" {
			t.Errorf("ImageConfig = %+v, expected 9:16 and 2K", config.ImageConfig)
		}
	})

	t.Run("should send only the size when the aspect ratio is unset", func(t *testing.T) {
		config := buildConfig(GenerateRequest{ImageSize: "4K"})
		if config.ImageConfig == nil {
			t.Fatal("ImageConfig is nil")
		}
		if config.ImageConfig.AspectRatio != "" || config.ImageConfig.ImageSize != "4K" {
			t.Errorf("ImageConfig = %+v, expected no aspect ratio and 4K", config.ImageConfig)
		}
	})

	t.Run("should send only the aspect ratio when the size is unset", func(t *testing.T) {
		config := buildConfig(GenerateRequest{AspectRatio: "1:1"})
		if config.ImageConfig == nil {
			t.Fatal("ImageConfig is nil")
		}
		if config.ImageConfig.AspectRatio != "1:1" || config.ImageConfig.ImageSize != "" {
			t.Errorf("ImageConfig = %+v, expected 1:1 and no size", config.ImageConfig)
		}
	})
}

func TestFinalImage(t *testing.T) {
	image := func(data string, thought bool) *genai.Part {
		return &genai.Part{InlineData: &genai.Blob{Data: []byte(data), MIMEType: "image/jpeg"}, Thought: thought}
	}
	text := func(s string, thought bool) *genai.Part {
		return &genai.Part{Text: s, Thought: thought}
	}
	response := func(parts ...*genai.Part) *genai.GenerateContentResponse {
		return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: &genai.Content{Parts: parts}}}}
	}

	t.Run("should return the only image", func(t *testing.T) {
		data, err := finalImage(response(image("final", false)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "final" {
			t.Errorf("image = %q, expected %q", data, "final")
		}
	})

	t.Run("should skip thought images and return the final image", func(t *testing.T) {
		data, err := finalImage(response(
			text("planning", true),
			image("thought 1", true),
			text("refining", true),
			image("thought 2", true),
			image("final", false),
		))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "final" {
			t.Errorf("image = %q, expected %q", data, "final")
		}
	})

	t.Run("should return the last non-thought image when there are several", func(t *testing.T) {
		data, err := finalImage(response(image("first", false), text("here you go", false), image("last", false)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "last" {
			t.Errorf("image = %q, expected %q", data, "last")
		}
	})

	t.Run("should return an error when there are only thought images", func(t *testing.T) {
		if _, err := finalImage(response(image("thought", true))); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("should return an error with the model's text when there is no image", func(t *testing.T) {
		_, err := finalImage(response(text("I can't make that image.", false)))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "I can't make that image.") {
			t.Errorf("err = %v, expected it to include the model's text", err)
		}
	})

	t.Run("should not include thought text in the error when there is no image", func(t *testing.T) {
		_, err := finalImage(response(text("secret plan", true), text("no image today", false)))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if strings.Contains(err.Error(), "secret plan") || !strings.Contains(err.Error(), "no image today") {
			t.Errorf("err = %v, expected only the non-thought text", err)
		}
	})

	t.Run("should return an error when there are no candidates", func(t *testing.T) {
		if _, err := finalImage(&genai.GenerateContentResponse{}); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("should return an error with the block reason when the prompt was blocked", func(t *testing.T) {
		_, err := finalImage(&genai.GenerateContentResponse{
			PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: genai.BlockedReasonProhibitedContent},
		})
		if err == nil || !strings.Contains(err.Error(), "PROHIBITED_CONTENT") {
			t.Errorf("err = %v, expected it to include the block reason", err)
		}
	})

	t.Run("should return an error with the finish reason and message when the candidate has no content", func(t *testing.T) {
		_, err := finalImage(&genai.GenerateContentResponse{Candidates: []*genai.Candidate{{
			FinishReason:  genai.FinishReason("IMAGE_SAFETY"),
			FinishMessage: "Unable to show the generated image.",
		}}})
		if err == nil || !strings.Contains(err.Error(), "IMAGE_SAFETY") || !strings.Contains(err.Error(), "Unable to show the generated image.") {
			t.Errorf("err = %v, expected it to include the finish reason and message", err)
		}
	})

	t.Run("should return an error with the finish reason when there is no image", func(t *testing.T) {
		res := response()
		res.Candidates[0].FinishReason = genai.FinishReason("NO_IMAGE")
		_, err := finalImage(res)
		if err == nil || !strings.Contains(err.Error(), "NO_IMAGE") {
			t.Errorf("err = %v, expected it to include the finish reason", err)
		}
	})
}
