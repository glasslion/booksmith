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

calibre must be installed. `booksmith` **never searches for it** — you tell it where
calibre lives, once, in `config.json` next to the binary:

```json
{ "calibre_dir": "C:\\Program Files\\Calibre2" }
```

A relative `calibre_dir` is resolved against the directory holding `config.json`. The
directory must contain `ebook-convert` and `calibre-debug` (`…exe` on Windows); a calibre
installation is a whole directory (Qt, Python, `app/`) and cannot be reduced to a single
executable. That is also why `booksmith` cannot bundle it.

Prefer not to keep a file around? Pass `--calibre DIR` instead.

If `config.json` is missing, `booksmith` prints the absolute path it wanted, a JSON
template you can paste verbatim, and the `--calibre` alternative.

## Usage

```
booksmith convert  [flags] <input>...
booksmith recover  [flags] <input>...
booksmith convert-recover [flags] <input>...
```

`<input>` may be a file, a directory, or a glob pattern. **Globs are expanded by
`booksmith`, not by the shell**, so `booksmith convert *.mobi` works in `cmd.exe` and
PowerShell too. Matching is case-insensitive on every platform, so `*.mobi` finds
`Book.MOBI`.

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
| `--calibre DIR` | all | the calibre directory (default: `calibre_dir` from `config.json`) |
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

Put the binary anywhere and give it a `config.json`:

```
booksmith.exe
config.json      { "calibre_dir": "C:\\Program Files\\Calibre2" }
```

On Windows `booksmith` switches the console output code page to UTF-8, so book titles and
calibre's own messages are readable in `cmd.exe` and Windows PowerShell (which otherwise
default to a legacy code page like 936).

### macOS

calibre's tools live inside the app bundle, so point `calibre_dir` there:

```bash
brew install --cask calibre
mkdir -p ~/bin/booksmith && cp dist/booksmith-darwin-arm64 ~/bin/booksmith/booksmith
printf '{ "calibre_dir": "/Applications/calibre.app/Contents/MacOS" }\n' \
  > ~/bin/booksmith/config.json
~/bin/booksmith/booksmith version
```

## License

MIT.
