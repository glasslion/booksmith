# Glossary

Terms are precise here on purpose. When code, tests, issues or ADRs name one of these
concepts they use the term exactly as defined below.

## Library-free

`booksmith` never opens, creates or reads a calibre **library** (the `metadata.db` +
per-book folder layout managed by the calibre GUI). It only uses calibre's standalone
command-line tools on individual files. Users keep their own way of organising books.

## Batch

One invocation over N **inputs**. Inputs come from positional arguments, which may be
files, directories or glob patterns (expanded by `booksmith`, not by the shell, because
`cmd.exe` and PowerShell do not expand globs). A directory contributes the ebook files it
directly contains unless `-r` is given. Each input produces exactly one **outcome**:
`ok`, `skipped` or `failed`.

## convert

The first subcommand. One format in, one format out via `ebook-convert`. Default target
is EPUB (`--to epub`); the target may be any format calibre supports.

## recover

The second subcommand. Produces a copy of an EPUB whose cover has been **stripped** and
then **regenerated** by calibre. Output is always a new file named
`<base>.recover.epub`; the input file is never modified.

## convert-recover

The third subcommand. Same as **recover**, but for non-EPUB inputs: the input is first
converted to EPUB, then stripped and regenerated, and the output is `<base>.epub`.
**EPUB inputs are skipped** — use **recover** for those. This subcommand never overwrites
the input file, because it never accepts one in the first place.

## strip cover

Removing the cover from an EPUB: the cover page document, the cover image it wraps, the
`<meta name="cover">` / `cover-image` markers, the `guide` reference and the spine entry.
Performed by calibre's own polish API — see ADR-0003.

## regenerate cover

Letting calibre create its default generated cover from the book's title and authors.
calibre does this automatically when converting an EPUB that has no cover, and logs
`Generating default cover`.

## sibling calibre directory

`booksmith` locates calibre by looking for `ebook-convert` (`ebook-convert.exe` on
Windows) in this order: `./calibre/` next to the executable, then the executable's own
directory, then `PATH`. A calibre installation is a whole directory (Qt, Python,
`app/`), so it is referenced, never copied. When the binary is found through `PATH`,
`booksmith` prints one line naming the resolved tool.
