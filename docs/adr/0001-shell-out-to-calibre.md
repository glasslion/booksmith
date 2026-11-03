# 0001 — Shell out to calibre instead of implementing conversion

- Status: Accepted
- Date: 2026-11-03

## Context

`booksmith` exists to make two batch operations bearable: MOBI → EPUB, and
"remove the cover, then regenerate it". Both are already solved by calibre. Writing an
EPUB/MOBI converter, or a cover renderer, would be a multi-year project with worse output
than the tool the user already trusts.

The user's only complaint about calibre is its library management model, not its
conversion engine. They explicitly want to keep the behaviour they know.

## Decision

`booksmith` is a thin, batch-oriented front end. It shells out to calibre's command-line
tools (`ebook-convert`, `calibre-debug`) and owns only the things calibre's own CLI is bad
at: globbing on Windows, per-file outcomes, output naming, and one-line progress.

calibre is a heavy directory (Qt, Python, `app/`), so it is never vendored into our
binary and never copied into the output. It is expected to live in `./calibre/` next to
the `booksmith` binary (or anywhere on `PATH`).

## Consequences

- We inherit calibre's format coverage, cover rendering and quirks — including its
  startup cost of roughly one second per process, which is why conversion runs serially.
- We must handle process invocation, output capture and error surfacing carefully: a
  calibre failure must never look like a `booksmith` failure, and vice versa.
- Users must install calibre themselves. `booksmith` fails loudly with an actionable
  message when it cannot find it, rather than shipping a broken conversion path.
