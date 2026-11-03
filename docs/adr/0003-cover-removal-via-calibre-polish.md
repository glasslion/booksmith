# 0003 — Cover removal is delegated to calibre's own polish API

- Status: Proposed (awaiting user sign-off)
- Date: 2026-11-03

## Context

`ebook-convert` cannot remove an existing cover. Verified on calibre 9.17.0 (Windows
build) with a hand-built EPUB2 fixture carrying `cover.jpg`, `cover.xhtml`,
`<meta name="cover">`, a `guide` reference and a spine entry:

| invocation | cover image kept | `<meta name="cover">` kept | cover page kept |
| --- | --- | --- | --- |
| `ebook-convert in.epub out.epub` | yes | yes | yes |
| `... --no-default-epub-cover` | yes | yes | yes |
| `... --remove-first-image` | yes | yes | yes |
| `... --remove-first-image --no-default-epub-cover` | yes | yes | yes |
| `ebook-convert in.epub out.epub --cover=none` | n/a — `none` is treated as a file path and fails |

`--no-default-epub-cover` only suppresses *generation*; `--remove-first-image` only acts
on an image that is not already identified as a cover. calibre has no CLI equivalent of
the library GUI's "Remove cover", which operates on the library database, not on the file.

Two ways to get a real strip:

- **A. Do the EPUB surgery ourselves** in Go (`archive/zip` + OPF rewriting), reproducing
  what calibre's polish module does: find cover page, find wrapped image, drop the
  manifest items, guide reference, `<meta name="cover">` and spine entry, then rezip.
- **B. Ask calibre to do it**, by running a small Python script through `calibre-debug -e`
  that calls `calibre.ebooks.oeb.polish`: `get_container`, `find_cover_page`,
  `find_cover_image`, `find_cover_image_in_page`, `remove_item`, `clean_opf`, `commit`.

Both were prototyped end to end against calibre 9.17.0; both produce a stripped EPUB that
`ebook-convert` then turns into one with a regenerated default cover
(`cover_image.jpg` + `titlepage.xhtml`, `Generating default cover` in the log).

## Decision

Use **B**. The user's stated requirement is to keep the calibre behaviour they already
know; option B *is* that code path, including its EPUB2/EPUB3 detection, SVG-wrapped
cover pages, `titlepage.xhtml` conventions and "extra cover page" heuristics, instead of
our reimplementation of a subset of them. Option A would silently diverge on the books
where calibre's heuristics are better than ours.

The script is embedded in the binary, written to a temporary file at run time and passed
to `calibre-debug -e` (found in the same directory as `ebook-convert`). It takes the
EPUB path as its last argument and prints a one-line report of what it removed.

Pipeline for `recover` / `convert-recover`:

1. Non-EPUB input → `ebook-convert` to a temporary EPUB.
2. Copy to a temporary working EPUB, run the strip script on it.
3. `ebook-convert` the stripped EPUB to its final format. calibre detects the missing
   cover and generates a default one from title + authors.
4. Move the result into place. On any failure, delete temporaries and leave the input
   untouched.

## Consequences

- We depend on calibre's internal Python API (`calibre.ebooks.oeb.polish`). A future
  calibre major could rename these functions. Mitigation: the call set is small and
  stable across 9.x, and a failure is reported per book with calibre's own stderr, so a
  version break degrades to a clear error rather than a corrupted file.
- No self-check of the produced cover: the user chose not to verify the output EPUB after
  conversion. A book whose strip or regeneration silently failed still counts as `ok`.
- Each book costs one extra calibre process (~1s) for the strip step.
- Option A is not implemented; if calibre's API breaks, this ADR is the place to reopen.
