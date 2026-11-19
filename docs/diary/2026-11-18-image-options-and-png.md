# Diary: Aspect ratio, resolution, multiple input images, and PNG output

The CLI uses only Nano Banana 2.1 (`gemini-nano-banana-2.1`) and writes JPEG only. The work behind that is in `/docs/diary/2026-11-18-current-nano-banana-models.md`. Google's image generation guide (https://aistudio.google.com/docs/image-generation) documents output options the CLI doesn't expose yet. This task adds the useful ones.

## Step 1: Read the image generation guide and agree on scope

**Author:** main

### Prompt Context

**Verbatim prompt:** Read the rest. Fix 1-3 and the PNG thing, and then commit, install and push
**Interpretation:** Add the three proposed features and PNG output, then commit, `go install`, and push straight to main.
**Inferred intent:** Make the CLI expose the model capabilities people actually use, without bloating it.

### What I did
I read the whole guide in three parts, because it is about 240k characters. It led me to propose four changes, and Markus accepted them:
1. An aspect ratio flag. The supported values for 2.1 are 1:1, 1:4, 1:8, 2:3, 3:2, 3:4, 4:1, 4:3, 4:5, 5:4, 8:1, 9:16, 16:9, and 21:9.
2. A resolution flag with the values 1K, 2K, and 4K. The API needs an uppercase K.
3. A repeatable `-i` flag. The model accepts up to 14 reference images: up to 10 objects and up to 4 characters.
4. PNG output again. The guide lists `image/png` as a supported `mime_type`, so the output format can follow the file extension.

I chose to skip thinking level, Google Search and Image Search grounding (Image Search requires showing search suggestions), video input, multi-turn interaction IDs, and batch.

### Why
Aspect ratio and size are the most common needs: square logos, phone wallpapers, and 4K prints. Multiple references enable compositing and character consistency. JPEG-only output was a workaround, not a goal.

### What worked
The guide has a full table of aspect ratios and pixel sizes for 2.1, which gives a definite list to validate against.

### What didn't work
WebFetch only reads 100k characters per call, so it took three calls (offsets 0, 100000, and 200000) to read the whole page. I also tried to write this diary and commit it in one compound shell command from the worktree. The sandbox refused it: "this command is too complex to verify that it stays inside the worktree".

### What I learned
The guide's Go examples use the new Interactions API (`client.Interactions.Create`, `interactions.ImageResponseFormat{AspectRatio, ImageSize}`). The CLI uses `Models.GenerateContent`. The guide also says Gemini 3 image models always think and can return up to two interim "thought" images before the final one, and the final image is the last. The CLI currently writes `resp.Images[0]`, so it may save a thought image.

### What was tricky
The guide doesn't say whether `GenerateContent` supports `outputMimeType` for this model. The builder has to check that against the live API.

### What warrants review
The choice between staying on `GenerateContent` and moving to the Interactions API.

### Future work
None yet.

## Step 2: Build the flags, multi-image input, PNG output, and thought-image handling

**Author:** builder

### Prompt Context

**Verbatim prompt:** You're the builder for a feature round in the nanobanana Go CLI (module `maragu.dev/nanobanana`). [...] Requirements (agreed with Markus): 1. `-aspect` [...] 2. `-size` [...] 3. Repeatable `-i` [...] 4. PNG output again [...] 5. Thought images [...] 6. API choice: stay on `Models.GenerateContent` if it supports aspect ratio, size and output MIME type for this model. [...] Move to the Interactions API [...] only if GenerateContent can't do it, and record why in the diary. [...] 7. Update the help text in /main.go and /README.md [...] 8. No telemetry; this is a CLI. (The lead's brief, abridged here; later amended with Markus's addition: "add examples to the README as needed [...] an aspect-ratio example [...] a multi-image compositing example, showing both inputs and the result [...] Commit them under /examples/ at 1K".)
**Interpretation:** Implement Step 1's four features plus correct final-image selection, pick the API path with evidence from the live API, verify with the built binary, add README examples from the live checks, and commit everything on the branch without pushing.
**Inferred intent:** Expose the model options people use, with local validation so mistakes fail fast and cost nothing, and make sure the file written is really the final image in the requested format.

### What I did
I checked the SDK before writing code. In `google.golang.org/genai` v1.38.0, `genai.ImageConfig` has `AspectRatio`, `ImageSize`, and `OutputMIMEType`, but the SDK's request converter for the Gemini Developer API (`imageConfigToMldev` in `models.go`) refuses the last one client-side: `outputMimeType parameter is not supported in Gemini API`. I bumped the SDK to v1.79.0 in the worktree to look further. It still refuses it ("outputMimeType parameter is only supported in Gemini Enterprise Agent Platform mode, not in Gemini Developer API mode."). v1.79.0 does have the Interactions API, and its `interactions.ImageResponseFormatMimeType` enum lists only `image/jpeg`.

Then I ran three throwaway probes against the live API (in a temporary `/cmd/probe` and `/cmd/probe2`, deleted afterwards):
1. `GenerateContent` with `ImageConfig{AspectRatio: "4:5", ImageSize: "1K"}`: one part, `Thought=false`, `image/jpeg`, 928x1152. So the newer aspect ratios that the SDK's doc comment doesn't list work.
2. Interactions API with `ImageResponseFormat{MimeType: "image/png", AspectRatio: "1:1", ImageSize: "1K"}`: rejected with `{"error":{"code":"invalid_request","message":"The value 'image/png' is not supported for 'response_format.mime_type'. Supported values: 'image/jpeg'."}}`.
3. `GenerateContent` (back on v1.38.0) with `ThinkingConfig{IncludeThoughts: true}`: four parts, thought text, a thought image (`Thought=true`, 1024x1024 JPEG), more thought text, then the final image (`Thought=false`, with a thought signature).

So I stayed on `Models.GenerateContent` and reverted the SDK bump (v1.38.0 supports everything the CLI sends). Then I wrote the tests first and made them pass:
- `/internal/nanobanana/client.go`: `GenerateRequest` now has `InputImages []nanobanana.Image` (data plus MIME type), `AspectRatio`, and `ImageSize`, and `GenerateResponse` has a single `Image`. Pure helpers `buildContents` (each image as its own inline part, in order, then the prompt, in one user turn), `buildConfig` (an `ImageConfig` only when aspect or size is set), and `finalImage` (skip `Part.Thought`, take the last image, include any model text in the error when there's no image) are tested in `/internal/nanobanana/client_internal_test.go`.
- `/main.go`: `parseGenerateArgs` parses and validates everything (aspect list, size in any case sent uppercase, at most 14 `-i` through a `stringsFlag` `flag.Value`, and output format from the extension) without touching disk or network. `readInputImage` reads each file and `detectImageMIMEType` checks it: `http.DetectContentType` for PNG, JPEG, and WebP, plus a check of the ISO BMFF `ftyp` major brand for HEIC and HEIF, which `http.DetectContentType` doesn't know. `toJPEG` became `convertImage(data, format)`, which sniffs the bytes, returns them unchanged on a match, and otherwise converts, still compositing onto white for JPEG.
- Updated the help text. A writer sub-agent updated `/README.md` from my brief, and I added `/examples/aspect-ratio-9-16.jpg` and `/examples/multiple-images.jpg` from the live checks.

Live checks with the built binary (outputs in the session scratchpad `options/` directory), each verified with `file`:
- (a) `generate a-default.png "a red paper airplane on a white background"`: PNG, 1376x768. After the bit-depth fix below, `generate -aspect 4:5 a-portrait.PNG ...`: `PNG image data, 928 x 1152, 8-bit/color RGB`.
- (b) `generate -aspect 1:1 -size 2k b-square-2k.jpg "a ripe banana on a marble kitchen counter, photorealistic"`: JPEG, 2048x2048. The lowercase `2k` got sent as `2K`.
- (c) `generate -aspect 9:16 c-portrait.jpg "a banana surfing a towering wave at sunset, photorealistic"`: JPEG, 768x1376.
- (d) `generate -i logo.jpg -i examples/nano-banana-2.1.jpg d-composite.jpg "put the wizard hat from the first image on the banana in the second"`: JPEG, 1376x768. The photo's top hat became the logo's starry wizard hat.
- (e) With `GOOGLE_API_KEY=not-a-real-key`, these all failed locally: `-aspect 7:3`, `-size 8k`, `e.webp`, `e` (no extension), 15 `-i`, a text file as `-i`, a 1-byte file as `-i`, and a missing `-i` file. A control run with no bad flags reached the API and got `API key not valid`. That shows the others never called it.

That makes eight live calls: three probes and five binary runs, with no 4K.

### Why
`GenerateContent` already does aspect ratio and size, and neither API path can produce PNG for this model, so moving to the Interactions API would add a second, generated client surface and gain nothing. Converting locally is the only route to PNG anyway. Keeping validation in one pure function means every local error happens before the API key is even used, and the tests can cover it without a network.

### What worked
Probing the live API with small throwaway programs settled the API question in three calls, instead of reasoning from SDK doc comments, which are out of date (the `AspectRatio` comment still lists only eight ratios). Running with a fake API key is a cheap, convincing way to show that validation runs before any network call.

### What didn't work
- The first PNG output was `PNG image data, 1376 x 768, 16-bit/color RGB` at 1.5 MB. `png.Encode` writes any image that isn't one of its known 8-bit types (such as the `*image.YCbCr` from decoding a JPEG) at 16 bits per channel. I added a failing test that checks the IHDR bit depth (`bit depth = 16, expected 8, because JPEG has 8 bits per channel`), then fixed it by drawing into an `*image.NRGBA` before encoding. The rerun gave 8-bit at about 0.9 MB for 928x1152.
- `go build ./cmd/probe2` failed with `new(ia.NewInteractionsInput(...)) requires go1.26 or later (-lang was set to go1.25; check go.mod)`. The module is on `go 1.25`, so `new(expr)` isn't available here.
- A compound `mkdir && cd scratchpad && go get ...` command was refused by the sandbox: "this command is too complex to verify that it stays inside the worktree". I bumped the SDK inside the worktree instead and reverted it with `git checkout go.mod go.sum`.

### What I learned
- The model returns JPEG over both APIs. "PNG output" means a lossless PNG of the model's JPEG, not extra fidelity.
- Thought images only appear in the response when `ThinkingConfig.IncludeThoughts` is set. The CLI doesn't set it, so the old `resp.Images[0]` happened to be right in practice. It would have saved a thought image as soon as someone turned thoughts on. `finalImage` handles both cases.
- The final image part carries a `ThoughtSignature` of about 1.5 MB, even though it isn't a thought.
- `http.DetectContentType` returns `text/plain; charset=utf-8` for a single `0xFF` byte, so the error for a 1-byte file says "detected type text/plain", which reads a bit oddly but is accurate to the sniffer.

### What was tricky
Requirement 6 said to stay on `GenerateContent` only if it supports the output MIME type as well. It doesn't (the SDK blocks it). But the Interactions API doesn't support PNG either, so moving would not have met the requirement. I treated "can't do it" as "can't do what we need", and staying was the outcome with less code for the same result.

### Self-review
I ran the fabrik:code-review skill: two competing reviewers over the diff. Neither found a serious issue. I fixed what both agreed on:
- `parseGenerateArgs` only checked `fs.NArg() < 2`, so `out.png "p" -size 4K` silently dropped the size and an unquoted `out.png a sunset` sent the prompt `a`, each a paid call with the wrong settings. It now needs exactly two positional arguments.
- The output directory wasn't checked until `os.WriteFile`, after the paid call. The handler now checks it after reading the inputs and before calling the API.
- Tests for `readInputImage` (the error names the file), a `Client.Generate` test against an `httptest` server that checks the request body (parts in order, then the prompt, and only `aspectRatio`/`imageSize` in `imageConfig`), and handler tests. In those tests a fake server fails the test on any request when an input or the output directory is bad, and the happy path writes the final image, not the thought image, as PNG.
- Blocked generations gave opaque errors ("no candidates in response"). `finalImage` now includes `PromptFeedback.BlockReason` and the candidate's `FinishReason`/`FinishMessage` (such as `IMAGE_SAFETY`, `NO_IMAGE`).
- The HEIF brand table was loose. An AVIF with major brand `mif1` was accepted as `image/heif`, and the sequence brands (`msf1`, `hevc`, `hevx`) were labelled `image/heif`. Now only `heic`/`heix`/`heim`/`heis` (HEIC) and `mif1` without an `avif`/`avis` compatible brand (HEIF) pass.
- `maxInputImages` was used in one place while `14` was hard-coded in the help and flag usage. It's now used in all of them.
- Comment fixes. The `convertImage` comment claimed the API "may return a different format than requested", but nothing is requested. The uppercase-size fact now lives on `nanobanana.GenerateRequest`. The `format` field is described as a MIME subtype. The `parseGenerateArgs` and `finalImage` docs are reworded. `-size` help now says the model decides (currently 1K), to match `-aspect`. A loop variable named `image` no longer shadows the `image` package. The tests use `strings.Contains`, and the table subtests have "should" names.

After the fixes I reran the local failure checks with the rebuilt binary and a fake key, adding `e.jpg a sunset` (unquoted prompt), `e.jpg "x" -size 4K`, and `missing/e.jpg`. All failed locally. I made no further live calls, so the five live binary runs above predate these fixes, but the fixes don't touch the request or response path that those runs exercised, and the `httptest` tests cover it.

I declined one point that both reviewers raised as moderate or optional: moving the allowed values and validation into `/internal/nanobanana`. The package has one caller, validation has to happen before the input files are read anyway, and the brief said not to restructure the CLI. I documented the constraints on `GenerateRequest` instead. Single-reviewer minor points I left alone: flag errors printing twice and `generate -h` returning an error (both predate this change), saving the raw bytes when conversion fails, and a dedicated empty-file message.

### What warrants review
- The accepted input types: PNG, JPEG, WebP, HEIC, HEIF, from Google's documented list. GIF and BMP are rejected locally even though they're images, because the API doesn't list them. HEIC/HEIF detection by `ftyp` brand is untested against the live API.
- `finalImage` takes the last non-thought image. If the model ever returns several final images, the CLI writes the last.
- The 16-bit PNG fix in `convertImage`, and that PNG→JPEG still composites onto white.

### Future work
None needed for this feature. If Google adds `image/png` to `response_format.mime_type` (or lifts the SDK's `outputMimeType` block), the CLI could request PNG natively and skip the conversion.

## Step 3: Document flag order in the help, and fix `generate -h` and doubled flag errors

**Author:** builder

### Prompt Context

**Verbatim prompt:** Small fix in the nanobanana Go CLI. [...] 1. **Flag order in the help.** The help text in /main.go (`helpHandler`) doesn't say that flags must come before the output path and the prompt should be quoted, but `parseGenerateArgs` now errors when they aren't. Add a short note under Usage [...] 2. **`nanobanana generate -h`.** Right now this prints Go's default flag usage ("Usage of generate: ...") and then `Error: failed to parse flags: flag: help requested`, with a failure exit. Make `generate -h` and `generate --help` print the same full help as `nanobanana help` and exit successfully. [...] While you're there, check whether flag errors are printed twice [...] 3. Keep everything else as it is: no new flags, and no help wording changes beyond the note. (Abridged.)
**Interpretation:** Add the two-line note to the help, route `-h`/`--help` on `generate` to `helpHandler`, and make the flag package stay silent so each error prints once.
**Inferred intent:** Close the two rough edges the Step 2 self-review left alone, without growing the CLI.

### What I did
In `/main.go`, `helpHandler` now has a note under Usage, mirroring the README: "Flags must come before the output path. Put the prompt in quotes." and "Any extra argument after the prompt, such as a flag, is a usage error." `parseGenerateArgs` calls `fs.SetOutput(io.Discard)` and returns `flag.ErrHelp` unwrapped; `generateHandler` checks `errors.Is(err, flag.ErrHelp)` and returns `helpHandler(ctx)`. Tests are in `/main_test.go`: `parseGenerateArgs` returns `flag.ErrHelp` for `-h`, `-help` and `--help`; `generateHandler` prints the same text as `helpHandler` with no error and no API call; a bad flag prints nothing from the handler or the flag package; and a `captureStderr` helper swaps `os.Stderr` for a file to check this.

### Why
The README already documented the flag order, and Step 2 made the CLI enforce it, so the help was the one place that didn't say so. A user who types `generate -h` expects help, not an error.

### What worked
Writing the tests first and disabling just the `SetOutput` line showed all three new output tests failing, so they do guard the fix. Flag errors were indeed printed twice: with `flag.ContinueOnError` and the default output, the flag package wrote the message and "Usage of generate:" to stderr, and `clir.Run` then printed `Error: ...` for the returned error. With the flag output discarded, `clir.Run` is the only printer.

### What didn't work
A first attempt to apply the edits with a Python heredoc and a chained `go doc | grep; go build` was refused by the sandbox as too complex to verify, so I used the edit tool and separate commands.

### What I learned
`flag.FlagSet` reads `os.Stderr` at print time when no output is set, so a test can swap `os.Stderr` to detect stray writes.

### What was tricky
Silencing the flag package also drops its list of valid flags on a bad flag. The error is now just `Error: failed to parse flags: flag provided but not defined: -nope`. I left it that way, since the brief said no help wording changes, and `generate -h` shows the flags.

### Self-review
The diff is small and I checked the cases directly: `-h`, `-help` and `--help` all work, a flag error prints once, and non-flag errors such as the invalid aspect ratio are unchanged. I did not run the competing-reviewer skill for a change of this size. A `-h` after other valid flags (`generate -size 4K -h`) also shows help, which is what the flag package does and seems right.

### What warrants review
The `errors.Is(err, flag.ErrHelp)` branch in `generateHandler` and the `SetOutput` line in `parseGenerateArgs`.

### Future work
None. If the lost valid-flags list on a bad flag turns out to matter, the error could point at `nanobanana generate -h`.
