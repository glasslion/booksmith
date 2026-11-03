# booksmith

A small batch front end for [calibre](https://calibre-ebook.com)'s command-line tools.

`booksmith` does three things, none of which calibre's own CLI does comfortably:

- **`convert`** — batch format conversion (`a.mobi` → `a.epub`).
- **`recover`** — strip an EPUB's cover, then let calibre generate a fresh one
  (`a.epub` → `a.recover.epub`).
- **`convert-recover`** — same, for non-EPUB inputs in one step (`a.mobi` → `a.epub`).

It never touches a calibre library. See [GLOSSARY.md](GLOSSARY.md) for the vocabulary and
[docs/adr](docs/adr) for the design decisions.

## Requirements

calibre must be installed. `booksmith` looks for `ebook-convert` (`ebook-convert.exe` on
Windows) in this order:

1. `./calibre/` next to the `booksmith` binary — the recommended layout, since a calibre
   installation is a whole directory (Qt, Python, `app/`) and cannot be reduced to a
   single executable;
2. the binary's own directory;
3. `PATH` — when this one is used, one line naming the resolved tool is printed.

## Usage

```
booksmith convert  [flags] <input>...
booksmith recover  [flags] <input>...
booksmith convert-recover [flags] <input>...
```

`<input>` may be a file, a directory, or a glob pattern. **Globs are expanded by
`booksmith`, not by the shell**, so `booksmith convert *.mobi` works in `cmd.exe` and
PowerShell too.

```bash
# convert a folder, recursively
booksmith convert -r ~/Books

# strip + regenerate covers, writing a.recover.epub next to each book
booksmith recover ~/Books/*.epub

# mobi → epub with a fresh cover, in one step (EPUB inputs are skipped)
booksmith convert-recover ~/Books/*.mobi

# see what would run, without running it
booksmith recover --dry-run ~/Books/*.epub
```

### Outputs

| subcommand | input | output | input touched? |
| --- | --- | --- | --- |
| `convert` | `a.mobi` | `a.epub` (or `a.<--to>`) | never |
| `recover` | `a.epub` | `a.recover.epub` | never |
| `convert-recover` | `a.mobi` | `a.epub` | never |
| `convert-recover` | `a.epub` | — (skipped) | never |

Existing output files are **skipped**, not overwritten. Nothing in `booksmith` is ever
destructive, and there is deliberately no `--in-place`.

### Flags

| flag | applies to | meaning |
| --- | --- | --- |
| `-r, --recursive` | all | descend into directories |
| `-o, --out DIR` | all | write outputs into `DIR` instead of next to the input |
| `--to FMT` | `convert` | target format, default `epub` |
| `--all` | all | don't restrict directory scans to ebook extensions |
| `--force` | `recover`, `convert-recover` | enable aggressive cover detection (first image in the book) when no cover marker exists |
| `--no-svg-cover` | `recover`, `convert-recover` | generate a plain `<img>` cover page instead of calibre's SVG wrapper (better compatibility with older readers) |
| `--keep-temp` | `recover`, `convert-recover` | keep the intermediate EPUBs for debugging |
| `--calibre DIR` | all | look for calibre in `DIR` instead of the default search order |
| `--dry-run` | all | print the calibre commands that would run |
| `-v, --verbose` | all | stream calibre's own output |

### Output and exit codes

One line per input:

```
D:\Books\dune.mobi -> D:\Books\dune.epub  ok 1.3s
D:\Books\emma.epub -> D:\Books\emma.recover.epub  ok 1.5s
D:\Books\same.epub -> D:\Books\same.epub  skip  would overwrite the input
D:\Books\broken.epub -> D:\Books\broken.recover.epub  fail  calibre: <calibre's last error line>
```

```
1 ok, 2 skipped, 1 failed
```

`--dry-run` prints the exact commands instead, including the internal copy step:

```
copy D:\Books\emma.epub <tmp>\work.epub &&
calibre-debug -e <tmp>\booksmith-strip-cover.py -- <tmp>\work.epub &&
ebook-convert <tmp>\work.epub <tmp>\final.epub
```

Exit code is `0` when nothing failed, `1` when at least one input failed, `2` on bad
usage. Failures never abort the batch.

## Build

Go 1.24+.

```bash
make build            # native
make windows          # → dist/booksmith.exe (amd64)
make mac              # → dist/booksmith-darwin-arm64, -amd64
make test
```

Put the binary next to a calibre directory and go:

```
booksmith.exe
calibre\
  ebook-convert.exe
  calibre-debug.exe
  app\ ...
```

## License

MIT.
