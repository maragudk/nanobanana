# nanobanana-cli

<img src="logo.png" alt="Logo" width="300" align="right">

[![Docs](https://pkg.go.dev/badge/maragu.dev/nanobanana)](https://pkg.go.dev/maragu.dev/nanobanana)

A Go CLI tool for generating and editing images using Google's Imagen 4 API.

⚠️ THIS APP IS VIBE-CODED BY AN AI, USE AT YOUR OWN RISK! ⚠️

Made with ✨sparkles✨ by [maragu](https://www.maragu.dev/): independent software consulting for cloud-native Go apps & AI engineering.

## Installation

```bash
go install maragu.dev/nanobanana/cmd/nanobanana@latest
```

Or build from source:

```bash
git clone https://github.com/maragudk/nanobanana-cli
cd nanobanana-cli
go build ./cmd/nanobanana
```

## Configuration

This tool uses Google's Gemini API with the Imagen 4 model. Set your Google API key as an environment variable:

```bash
export GOOGLE_API_KEY="your-google-api-key-here"
```

Or create a `.env` file in your working directory:

```bash
GOOGLE_API_KEY=your-google-api-key-here
```

The CLI will automatically load the `.env` file if it exists.

Get your API key from [Google AI Studio](https://makersuite.google.com/app/apikey).

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
# Generate a photorealistic image
nanobanana generate photo.png "a photorealistic portrait of a cat"

# Edit an image
nanobanana generate -i photo.png edited.png "add sunglasses to the cat"

# Generate multiple variations
nanobanana generate -count 5 variations.png "abstract art with vibrant colors"
```

## License

[MIT](LICENSE)

[Contact me at markus@maragu.dk](mailto:markus@maragu.dk) for consulting work, or perhaps an invoice to support this project?
