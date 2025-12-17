# nanobanana

<img src="logo.png" alt="Logo" width="300" align="right">

[![Docs](https://pkg.go.dev/badge/maragu.dev/nanobanana)](https://pkg.go.dev/maragu.dev/nanobanana)

A Go CLI tool for generating and editing images using Google's Gemini image generation models (Nano Banana and Nano Banana Pro).

⚠️ THIS APP IS VIBE-CODED BY AN AI, USE AT YOUR OWN RISK! ⚠️

Made with ✨sparkles✨ by [maragu](https://www.maragu.dev/): independent software consulting for cloud-native Go apps & AI engineering.

## Installation

```bash
go install maragu.dev/nanobanana@latest
```

Or build from source:

```bash
git clone https://github.com/maragudk/nanobanana
cd nanobanana
go build
```

## Configuration

This tool uses Google's Gemini API with two image generation models:

- **Nano Banana** (`gemini-2.5-flash-image`) - Fast, cost-effective image generation (default)
- **Nano Banana Pro** (`gemini-3-pro-image-preview`) - Higher quality with advanced features like better text rendering, higher resolution (up to 4K), and Google Search grounding

Set your Google API key as an environment variable:

```bash
export GOOGLE_API_KEY="your-google-api-key-here"
```

Or create a `.env` file in your working directory:

```bash
GOOGLE_API_KEY=your-google-api-key-here
```

The CLI will automatically load the `.env` file if it exists.

Get your API key from [Google AI Studio](https://makersuite.google.com/app/apikey).

## Supported Formats

The CLI automatically detects the output format from the file extension:

- **PNG** (`.png`) - Default format, lossless compression
- **JPEG** (`.jpg`, `.jpeg`) - Lossy compression, smaller file sizes

If no extension is provided, PNG format is used by default.

```bash
nanobanana generate image.png "a sunset"  # PNG format
nanobanana generate image.jpg "a sunset"  # JPEG format
```

## Usage

### Generate an image

Generate an image from a text prompt:

```bash
nanobanana generate output.png "a beautiful sunset over mountains"
```

Generate multiple variations:

```bash
nanobanana generate -count 3 output.png "a beautiful sunset over mountains"
```

### Use Nano Banana Pro for higher quality

Use the `-pro` flag to generate images with Nano Banana Pro for higher quality output:

```bash
nanobanana generate -pro output.png "professional product photo"
```

Nano Banana Pro is recommended when you need:
- High-resolution output (up to 4K)
- Legible text in images (infographics, menus, diagrams)
- Professional-quality images
- Complex multi-turn editing workflows

Note: Nano Banana Pro is slower and more expensive than the standard model, but produces significantly better results for professional use cases.

### Edit an existing image

Edit an image using natural language instructions:

```bash
nanobanana generate -i input.png output.png "make the sky more purple"
```

## Examples

```bash
# Generate a photorealistic image (PNG)
nanobanana generate photo.png "a photorealistic portrait of a cat"

# Generate a JPEG image
nanobanana generate photo.jpg "a photorealistic portrait of a cat"

# Edit an image
nanobanana generate -i photo.png edited.png "add sunglasses to the cat"

# Generate multiple variations
nanobanana generate -count 5 variations.png "abstract art with vibrant colors"

# Use Nano Banana Pro for high-quality professional images
nanobanana generate -pro output.png "modern infographic about climate change with legible text"

# Edit with Nano Banana Pro for better results
nanobanana generate -pro -i logo.png logo-wizard.png "add a little wizard hat to the banana"
```
