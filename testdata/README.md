# testdata

Real books go here, and they are **not committed** (see `.gitignore`).

Drop EPUB/MOBI files into this directory to exercise the real thing:

```bash
# a real conversion, using a real calibre
./dist/booksmith convert testdata/*.mobi

# a real cover strip + regenerate
./dist/booksmith recover testdata/*.epub
```

The automated tests do not need these files: they use generated fixtures and a stub
calibre. Nothing here is read by `go test`.
