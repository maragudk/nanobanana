# nanobanana

<img src="logo.png" alt="Logo" width="300" align="right">

[![Docs](https://pkg.go.dev/badge/maragu.dev/nanobanana)](https://pkg.go.dev/maragu.dev/nanobanana)

A Go CLI tool for generating and editing images using Google's Gemini Flash 2.5 Image API.

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

This tool uses Google's Gemini API with the Gemini Flash 2.5 Image model. Set your Google API key as an environment variable:

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
```

## License

[MIT](LICENSE)

[Contact me at markus@maragu.dk](mailto:markus@maragu.dk) for consulting work, or perhaps an invoice to support this project?
