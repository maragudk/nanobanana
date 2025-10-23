package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"maragu.dev/clir"
	"maragu.dev/env"
	"maragu.dev/errors"

	"maragu.dev/nanobanana/internal/nanobanana"
)

func main() {
	// Load .env file if it exists (ignore errors if it doesn't)
	_ = env.Load(".env")

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		os.Stderr.WriteString("Error: GOOGLE_API_KEY environment variable is required\n\n")
		os.Stderr.WriteString("To use nanobanana, you need a Google API key:\n")
		os.Stderr.WriteString("  1. Get your API key from https://makersuite.google.com/app/apikey\n")
		os.Stderr.WriteString("  2. Set it as an environment variable:\n")
		os.Stderr.WriteString("       export GOOGLE_API_KEY=\"your-api-key-here\"\n")
		os.Stderr.WriteString("  3. Or create a .env file:\n")
		os.Stderr.WriteString("       echo \"GOOGLE_API_KEY=your-api-key-here\" > .env\n")
		os.Exit(1)
	}

	client := nanobanana.NewClient(apiKey)

	router := clir.NewRouter()
	router.RouteFunc("", helpHandler)
	router.RouteFunc("-h", helpHandler)
	router.RouteFunc("--help", helpHandler)
	router.RouteFunc("help", helpHandler)
	router.RouteFunc("generate", generateHandler(client))

	clir.Run(router)
}

func helpHandler(ctx clir.Context) error {
	ctx.Println("nanobanana - CLI for Nano Banana image generation API")
	ctx.Println("")
	ctx.Println("Usage:")
	ctx.Println("  nanobanana generate [flags] <output-path> <prompt>")
	ctx.Println("")
	ctx.Println("Commands:")
	ctx.Println("  generate    Generate or edit images")
	ctx.Println("")
	ctx.Println("Examples:")
	ctx.Println("  # Generate an image")
	ctx.Println("  nanobanana generate output.png \"a beautiful sunset over mountains\"")
	ctx.Println("")
	ctx.Println("  # Generate multiple variations")
	ctx.Println("  nanobanana generate -count 3 output.png \"abstract art\"")
	ctx.Println("")
	ctx.Println("  # Edit an existing image")
	ctx.Println("  nanobanana generate -i input.png output.png \"make the sky purple\"")
	ctx.Println("")
	ctx.Println("Flags:")
	ctx.Println("  -i string     Input image path for editing")
	ctx.Println("  -count int    Number of images to generate (default 1)")
	ctx.Println("")
	ctx.Println("Configuration:")
	ctx.Println("  Set GOOGLE_API_KEY environment variable or create a .env file")
	return nil
}

func generateHandler(client *nanobanana.Client) clir.RunnerFunc {
	return func(ctx clir.Context) error {
		fs := flag.NewFlagSet("generate", flag.ContinueOnError)
		inputImage := fs.String("i", "", "input image path for editing")
		count := fs.Int("count", 1, "number of images to generate")

		if err := fs.Parse(ctx.Args); err != nil {
			return errors.Wrap(err, "failed to parse flags")
		}

		// Expect: [output-path] [prompt/instructions]
		if fs.NArg() < 2 {
			return errors.New("usage: nanobanana generate [-i input-image] [-count N] <output-path> <prompt>")
		}

		outputPath := fs.Arg(0)
		prompt := fs.Arg(1)

		req := nanobanana.GenerateRequest{
			Prompt:         prompt,
			Count:          *count,
			OutputMIMEType: mimeTypeFromExtension(outputPath),
		}

		// If -i flag is set, read the input image
		if *inputImage != "" {
			imageData, err := os.ReadFile(*inputImage)
			if err != nil {
				return errors.Wrap(err, "failed to read input image")
			}
			req.InputImage = bytes.NewReader(imageData)
		}

		resp, err := client.Generate(ctx.Ctx, req)
		if err != nil {
			return errors.Wrap(err, "failed to generate image")
		}

		if len(resp.Images) == 0 {
			return errors.New("no images returned from API")
		}

		// Write the first image to the output path
		if err := os.WriteFile(outputPath, resp.Images[0], 0644); err != nil {
			return errors.Wrap(err, "failed to write output image")
		}

		if *inputImage != "" {
			ctx.Printfln("Successfully edited image: %s", outputPath)
		} else {
			ctx.Printfln("Successfully generated image: %s", outputPath)
		}

		return nil
	}
}

// mimeTypeFromExtension returns the MIME type based on the file extension.
// Supported formats: png (default), jpg/jpeg
func mimeTypeFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		// Default to PNG if extension is unknown or missing
		return "image/png"
	}
}
