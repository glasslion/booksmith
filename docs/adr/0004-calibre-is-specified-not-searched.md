# 0004 — calibre is specified, never searched for; no bundling

- Status: Accepted
- Date: 2026-11-03

## Context

The tool needed to stop asking users for a calibre path on every invocation. Three
approaches were on the table: probe for calibre automatically, ship calibre with
`booksmith`, or record the path in a config file.

Two experiments decided the bundling question, both on calibre 9.17.0 (Windows):

1. **calibre's CLI tools are not standalone.** `ebook-convert.exe`, `ebook-meta.exe`,
   `ebook-polish.exe` and `calibre-debug.exe` are all 36,104-byte launchers; the payload
   is the whole install (`python-lib.bypy.frozen` 84 MB, 167 compiled extensions, Qt,
   resources). A lone `ebook-convert.exe` in an empty directory exits **1 with no output
   at all** — it cannot even report its own breakage. Removing
   `app/resources/localization` (18 MB of ISO-639 data) makes every invocation fail at
   import time, from `ebook-convert`'s own CLI module.
2. **Trimming trades correctness for megabytes.** 669 MB → 559 MB (drop translations,
   devtools resources, onnxruntime, DirectML) keeps output identical. Dropping
   `Qt6WebEngineCore.dll` (192 MB) keeps every command exiting 0, but the generated
   cover silently degrades from calibre's decorative frame to a plain gradient with
   text. There is no small, correct calibre.

Separately, the MSI is per-machine (`ALLUSERS=1`) and installs to
`[ProgramFiles64Folder]\Calibre2`, but `ApplicationFolderName` is user-changeable, so a
default-install probe would still miss custom installs.

## Decision

- **Ship no calibre.** Users install it from calibre's own download page.
- **Probe nothing.** The previous chain (`./calibre`, next to the binary, `PATH`) is
  deleted. calibre's location comes from `--calibre DIR`, else `calibre_dir` in
  `config.json` next to the binary, else an error. No `PATH`, no standard install
  locations, no environment variable.
- **The error is the onboarding.** With no config, the message prints the absolute path
  it wanted, a JSON template to paste verbatim, and the `--calibre` alternative. Three
  distinct messages cover: no config, invalid config, directory without the tools.
- **The config lives next to the binary**, so a copied folder carries its settings, and
  one config can be shared by every shell and directory.
- **No config subcommand.** The file is one key; the user writes it.
- **No diagnostics command.** This is a personal tool; when calibre is broken, the user
  can tell it is calibre's problem.

## Consequences

- Every machine needs one line of config (or `--calibre`). In exchange there is exactly
  one way calibre is found, so "why is it picking that one?" cannot happen.
- A stale `calibre_dir` is a hard error rather than a silent fallback (deliberate: a
  fallback would hide the mistake).
- We lost the "unzip and go" property that `./calibre/` gave us; that trade was made
  knowingly, because the alternative was a 240 MB download full of GPL binaries and
  per-platform signing work.
- Legacy: `--calibre` used to accept a parent directory and probe `DIR/calibre`. It now
  means the calibre directory itself.
- The GLOSSARY term "sibling calibre directory" is gone, replaced by **calibre_dir** and
  **config file**.
