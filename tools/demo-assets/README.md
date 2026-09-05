# Demo asset authoring tool

Maintainer-only utility for refreshing the curated real photos/videos used by
the demo workspace (`internal/demo/assets/`). Not part of the server build,
not run by `go generate`, and not run in CI.

## What it does

`fetch.sh` downloads a hand-picked list of Wikimedia Commons files (real
photos + short video clips, all under CC0/CC-BY/CC-BY-SA/public-domain
licenses, see `../../internal/demo/assets/ATTRIBUTION.md`), then:

- re-encodes photos to reasonable web dimensions (JPEG, `ffmpeg -q:v 3`)
- crops the four social-format slots to their platform aspect ratio
- mutes and re-encodes the two video clips to small h264 MP4s (muting avoids
  any question about rights to a video's original audio/music track, the
  demo doesn't need sound)
- generates the three logo files as original geometric marks (not stock
  photography, this is a fictional client's brand)
- writes synthetic-but-plausible EXIF into the real JPEGs (camera, lens,
  exposure, backdated `DateTimeOriginal`, and GPS on exactly two photos) so
  the EXIF extraction feature (`ROADMAP_14_exif.md`) has something to show
  in the demo

Raw downloads are cached in `.cache/` (gitignored) so re-runs don't re-hit
Wikimedia unless you delete the cache.

## Requirements

`curl`, `ffmpeg`/`ffprobe`, `exiftool`, and ImageMagick (`convert`) on `PATH`.

## Usage

```bash
tools/demo-assets/fetch.sh
```

This overwrites `internal/demo/assets/photos/` and
`internal/demo/assets/videos/` in place. Review the result, then:

1. If you changed what any slot actually shows, update the matching entry in
   `internal/demo/assets/manifest.yaml`, `tags`, `fields`, and especially
   `alt` must describe what's genuinely in the new file (see RA-4 in
   `ROADMAP.79.demo-real-assets.md`; `go test -tags demo ./internal/demo/...`
   catches an unknown tag/field/project reference or a missing file, but it
   can't catch "the alt text lies about the picture").
2. Update `internal/demo/assets/ATTRIBUTION.md` with the new file's title,
   author, and license (fetch it from the file's Commons page).
3. Re-run `go test -tags demo ./internal/demo/...` to confirm the manifest
   still validates and the seeder still builds every asset.

## Swapping in a different source photo

Edit the `download` list at the top of `fetch.sh` (Commons file URL + a
short cache name), point the matching `ffmpeg`/`convert` command at the new
cache name, then follow steps 1–3 above.
