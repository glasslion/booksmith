# 0002 — Three subcommands: `convert`, `recover`, `convert-recover`

- Status: Accepted
- Date: 2026-11-03

## Context

Two jobs need doing (see GLOSSARY: **convert**, **recover**), but they differ in input
format, output format and output filename. A single command with mode flags would make
the output-naming rules conditional and hard to remember:

| input | job | output |
| --- | --- | --- |
| `a.mobi` | format change | `a.epub` |
| `a.epub` | new cover | `a.recover.epub` |
| `a.mobi` | new cover | `a.epub` |

## Decision

Three subcommands, sharing one batch/plumbing layer:

- `convert` — format change only. Output `<base>.<to>`, default `to=epub`.
- `recover` — strip + regenerate cover. Output always `<base>.recover.epub`. The input
  file is never modified.
- `convert-recover` — strip + regenerate for non-EPUB inputs, output `<base>.epub`.
  **EPUB inputs are skipped** by this subcommand; `recover` is the tool for those.

`convert-recover` exists because a MOBI book that needs a new cover would otherwise need
two invocations and an intermediate filename the user has to invent. It skips EPUBs
because that would mean writing to `<base>.epub`, i.e. overwriting the input.

## Consequences

- Every subcommand is non-destructive: no command in `booksmith` ever writes over an
  existing input file. There is deliberately no `--in-place`.
- The output filename is fully determined by (subcommand, input), so batches are
  predictable and re-runnable.
- `recover` and `convert-recover` share their cover logic entirely; only the input filter
  and output naming differ.
- Existing outputs are skipped rather than overwritten (see the batch rules in README).
