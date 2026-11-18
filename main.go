package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
		fmt.Fprintf(os.Stderr, "Error: GOOGLE_API_KEY environment variable is required\n\n")
		fmt.Fprintf(os.Stderr, "To use nanobanana, you need a Google API key:\n")
		fmt.Fprintf(os.Stderr, "  1. Get your API key from https://aistudio.google.com/apikey\n")
		fmt.Fprintf(os.Stderr, "  2. Set it as an environment variable:\n")
		fmt.Fprintf(os.Stderr, "       export GOOGLE_API_KEY=\"your-api-key-here\"\n")
		fmt.Fprintf(os.Stderr, "  3. Or create a .env file:\n")
		fmt.Fprintf(os.Stderr, "       echo \"GOOGLE_API_KEY=your-api-key-here\" > .env\n")
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
	ctx.Println("  # Generate a square 4K image")
	ctx.Println("  nanobanana generate -aspect 1:1 -size 4K output.jpg \"a minimalist logo of a banana\"")
	ctx.Println("")
	ctx.Println("  # Edit an existing image")
	ctx.Println("  nanobanana generate -i input.png output.jpg \"make the sky purple\"")
	ctx.Println("")
	ctx.Println("  # Combine several images")
	ctx.Println("  nanobanana generate -i hat.png -i banana.jpg output.png \"put the hat from the first image on the banana in the second\"")
	ctx.Println("")
	ctx.Println("Output:")
	ctx.Println("  The output format follows the extension of the output path:")
	ctx.Println("  .png gives PNG, and .jpg or .jpeg gives JPEG.")
	ctx.Println("")
	ctx.Println("Flags:")
	ctx.Printfln("  -i string       Input image path. Repeat for up to %v images.", maxInputImages)
	ctx.Println("                  Input images can be PNG, JPEG, WebP, HEIC, or HEIF.")
	ctx.Println("  -aspect string  Aspect ratio: " + strings.Join(aspectRatios, ", "))
	ctx.Println("                  If not set, the model decides.")
	ctx.Println("  -size string    Image size: " + strings.Join(imageSizes, ", ") + ", in any case")
	ctx.Println("                  If not set, the model decides (currently 1K).")
	ctx.Println("")
	ctx.Println("Configuration:")
	ctx.Println("  Set GOOGLE_API_KEY environment variable or create a .env file")
	return nil
}

func generateHandler(client *nanobanana.Client) clir.RunnerFunc {
	return func(ctx clir.Context) error {
		opts, err := parseGenerateArgs(ctx.Args)
		if err != nil {
			return err
		}

		req := nanobanana.GenerateRequest{
			Prompt:      opts.prompt,
			AspectRatio: opts.aspectRatio,
			ImageSize:   opts.imageSize,
		}

		for _, path := range opts.inputPaths {
			input, err := readInputImage(path)
			if err != nil {
				return err
			}
			req.InputImages = append(req.InputImages, input)
		}

		// Check the output directory now, so a paid result isn't lost at the end.
		dir := filepath.Dir(opts.outputPath)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return errors.Newf("output directory %q doesn't exist", dir)
		}

		res, err := client.Generate(ctx.Ctx, req)
		if err != nil {
			return errors.Wrap(err, "failed to generate image")
		}

		imageData, err := convertImage(res.Image, opts.format)
		if err != nil {
			return errors.Wrap(err, "failed to convert image to %v", strings.ToUpper(opts.format))
		}

		if err := os.WriteFile(opts.outputPath, imageData, 0644); err != nil {
			return errors.Wrap(err, "failed to write output image")
		}

		if len(opts.inputPaths) > 0 {
			ctx.Printfln("Successfully edited image: %s", opts.outputPath)
		} else {
			ctx.Printfln("Successfully generated image: %s", opts.outputPath)
		}

		return nil
	}
}

// aspectRatios supported by the model.
var aspectRatios = []string{"1:1", "1:4", "1:8", "2:3", "3:2", "3:4", "4:1", "4:3", "4:5", "5:4", "8:1", "9:16", "16:9", "21:9"}

// imageSizes supported by the model.
var imageSizes = []string{"1K", "2K", "4K"}

// maxInputImages the model accepts in one request.
const maxInputImages = 14

// generateOptions are the validated arguments to the generate command.
type generateOptions struct {
	outputPath  string
	prompt      string
	inputPaths  []string
	aspectRatio string
	imageSize   string
	// format of the output image, as a MIME subtype: "png" or "jpeg".
	format string
}

// stringsFlag is a [flag.Value] that collects every value of a repeated flag.
type stringsFlag []string

func (s *stringsFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringsFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// parseGenerateArgs for the generate command and validate them, without touching the file system or the network.
func parseGenerateArgs(args []string) (generateOptions, error) {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var inputPaths stringsFlag
	fs.Var(&inputPaths, "i", fmt.Sprintf("input image path, repeat for up to %v images", maxInputImages))
	aspectRatio := fs.String("aspect", "", "aspect ratio, such as 1:1 or 16:9")
	imageSize := fs.String("size", "", "image size: "+strings.Join(imageSizes, ", "))

	if err := fs.Parse(args); err != nil {
		return generateOptions{}, errors.Wrap(err, "failed to parse flags")
	}

	// Expect exactly [output-path] [prompt], so flags after them, or an unquoted prompt, aren't silently dropped.
	if fs.NArg() != 2 {
		return generateOptions{}, errors.New("usage: nanobanana generate [-i input-image]... [-aspect ratio] [-size size] <output-path> <prompt>")
	}

	opts := generateOptions{
		outputPath:  fs.Arg(0),
		prompt:      fs.Arg(1),
		inputPaths:  inputPaths,
		aspectRatio: *aspectRatio,
		// Any case is accepted here, but [nanobanana.GenerateRequest] needs the size in uppercase.
		imageSize: strings.ToUpper(*imageSize),
	}

	if len(opts.inputPaths) > maxInputImages {
		return generateOptions{}, errors.Newf("too many input images (%v), the maximum is %v", len(opts.inputPaths), maxInputImages)
	}

	if opts.aspectRatio != "" && !slices.Contains(aspectRatios, opts.aspectRatio) {
		return generateOptions{}, errors.Newf("invalid aspect ratio %q, must be one of: %v", opts.aspectRatio, strings.Join(aspectRatios, ", "))
	}

	if opts.imageSize != "" && !slices.Contains(imageSizes, opts.imageSize) {
		return generateOptions{}, errors.Newf("invalid image size %q, must be one of: %v", *imageSize, strings.Join(imageSizes, ", "))
	}

	switch strings.ToLower(filepath.Ext(opts.outputPath)) {
	case ".png":
		opts.format = "png"
	case ".jpg", ".jpeg":
		opts.format = "jpeg"
	default:
		return generateOptions{}, errors.Newf("output path %q must end in .png, .jpg, or .jpeg", opts.outputPath)
	}

	return opts, nil
}

// readInputImage at path and detect its MIME type, returning an error naming the file if it isn't a supported image.
func readInputImage(path string) (nanobanana.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nanobanana.Image{}, errors.Wrap(err, "failed to read input image")
	}

	mimeType, err := detectImageMIMEType(data)
	if err != nil {
		return nanobanana.Image{}, errors.Wrap(err, "invalid input image %q", path)
	}

	return nanobanana.Image{Data: data, MIMEType: mimeType}, nil
}

// detectImageMIMEType from the content of data, returning an error unless it's an image type the model accepts.
func detectImageMIMEType(data []byte) (string, error) {
	// HEIC and HEIF are ISO base media files, which [http.DetectContentType] doesn't recognize,
	// so check the brands in the leading "ftyp" box: the major brand at offset 8, and compatible brands from offset 16.
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		switch string(data[8:12]) {
		case "heic", "heix", "heim", "heis":
			return "image/heic", nil
		case "mif1":
			// AVIF also uses mif1 as its major brand, and is told apart by its compatible brands.
			boxSize := min(int(binary.BigEndian.Uint32(data[0:4])), len(data))
			for i := 16; i+4 <= boxSize; i += 4 {
				if brand := string(data[i : i+4]); brand == "avif" || brand == "avis" {
					return "", errors.New("detected type image/avif, but input images must be PNG, JPEG, WebP, HEIC, or HEIF")
				}
			}
			return "image/heif", nil
		}
	}

	mimeType := http.DetectContentType(data)
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp":
		return mimeType, nil
	}
	return "", errors.Newf("detected type %v, but input images must be PNG, JPEG, WebP, HEIC, or HEIF", mimeType)
}

// convertImage to format ("png" or "jpeg"), returning data unchanged if it already is in that format.
// The current format is sniffed from data.
// Converting to JPEG composites the image onto white, because JPEG has no alpha channel.
func convertImage(data []byte, format string) ([]byte, error) {
	if http.DetectContentType(data) == "image/"+format {
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode image")
	}

	var buf bytes.Buffer
	switch format {
	case "png":
		// Without converting to NRGBA, a decoded JPEG would be encoded with 16 bits per channel, doubling the size for nothing.
		nrgba := image.NewNRGBA(img.Bounds())
		draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)

		if err := png.Encode(&buf, nrgba); err != nil {
			return nil, errors.Wrap(err, "failed to encode PNG")
		}
	case "jpeg":
		// Without compositing, transparent pixels would turn black.
		flat := image.NewRGBA(img.Bounds())
		draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)

		if err := jpeg.Encode(&buf, flat, nil); err != nil {
			return nil, errors.Wrap(err, "failed to encode JPEG")
		}
	default:
		return nil, errors.Newf("unsupported output format %q", format)
	}
	return buf.Bytes(), nil
}
