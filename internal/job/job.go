// Package job plans and executes batches of ebook jobs.
package job

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"booksmith/internal/batch"
	"booksmith/internal/calibre"
	"booksmith/internal/cover"
)

// Mode is which subcommand is running.
type Mode int

const (
	Convert Mode = iota
	Recover
	ConvertRecover
)

// Config is everything the batch layer needs to know about the invocation.
type Config struct {
	Mode       Mode
	To         string // convert only; default "epub"
	OutDir     string // optional; outputs go here instead of next to the input
	NoSVGCover bool
	KeepTemp   bool
	DryRun     bool
}

// Plan is one input, its intended output, and why it may be skipped.
type Plan struct {
	In   string
	Out  string
	Skip string
}

// Build computes a plan per input. Skip reasons are static (they depend only on paths),
// so a dry run shows exactly what a real run would do.
func Build(cfg Config, inputs []string) []Plan {
	plans := make([]Plan, 0, len(inputs))
	for _, in := range inputs {
		p := Plan{In: in}
		switch cfg.Mode {
		case Convert:
			p.Out = sibling(in, "."+cfg.targetExt())
			if sameFile(p.Out, in) {
				p.Skip = "would overwrite the input"
			}
		case Recover:
			if !batch.IsEpub(in) {
				p.Skip = "not an epub (use convert-recover)"
			}
			p.Out = sibling(in, ".recover.epub")
		case ConvertRecover:
			if batch.IsEpub(in) {
				p.Skip = "epub input (use recover)"
			}
			p.Out = sibling(in, ".epub")
			if p.Skip == "" && sameFile(p.Out, in) {
				p.Skip = "would overwrite the input"
			}
		}
		if cfg.OutDir != "" {
			p.Out = filepath.Join(cfg.OutDir, filepath.Base(p.Out))
		}
		if p.Skip == "" {
			if _, err := os.Stat(p.Out); err == nil {
				p.Skip = "exists"
			}
		}
		plans = append(plans, p)
	}
	return plans
}

// sibling returns path with its final extension replaced by suffix, in the same directory.
func sibling(path, suffix string) string {
	return filepath.Join(filepath.Dir(path), batch.Base(path)+suffix)
}

func (c Config) targetExt() string {
	to := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(c.To)), ".")
	if to == "" {
		to = "epub"
	}
	return to
}

func sameFile(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return aa == bb
}

// Status is how one input ended up.
type Status string

const (
	StatusOK      Status = "ok"
	StatusSkipped Status = "skip"
	StatusFailed  Status = "fail"
	StatusDryRun  Status = "dry-run"
)

// Result is one input's outcome.
type Result struct {
	Plan   Plan
	Status Status
	Note   string
	Dur    time.Duration
}

// Summary is every outcome in a batch.
type Summary struct {
	Results []Result
}

func (s Summary) count(status Status) int {
	n := 0
	for _, r := range s.Results {
		if r.Status == status {
			n++
		}
	}
	return n
}

// OK is how many inputs succeeded.
func (s Summary) OK() int { return s.count(StatusOK) }

// Skipped is how many inputs were skipped.
func (s Summary) Skipped() int { return s.count(StatusSkipped) }

// Failed is how many inputs failed.
func (s Summary) Failed() int { return s.count(StatusFailed) }

// ExitCode is 1 when anything failed, else 0.
func (s Summary) ExitCode() int {
	if s.Failed() > 0 {
		return 1
	}
	return 0
}

// String renders the trailing summary line.
func (s Summary) String() string {
	parts := make([]string, 0, 3)
	if n := s.OK(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d ok", n))
	}
	if n := s.Skipped(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", n))
	}
	if n := s.Failed(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", n))
	}
	if n := s.count(StatusDryRun); n > 0 {
		parts = append(parts, fmt.Sprintf("%d dry-run", n))
	}
	if len(parts) == 0 {
		return "nothing to do"
	}
	return strings.Join(parts, ", ")
}

// Execute runs every plan, printing one line each. Failures never abort the batch.
func Execute(ctx context.Context, r calibre.Runner, tools calibre.Tools, cfg Config, plans []Plan, w io.Writer) Summary {
	var s Summary
	for _, plan := range plans {
		start := time.Now()
		res := Result{Plan: plan}
		switch {
		case plan.Skip != "":
			res.Status = StatusSkipped
			res.Note = plan.Skip
		case cfg.DryRun:
			res.Status = StatusDryRun
			res.Note = dryRunLine(cfg, plan, tools)
		default:
			note, err := runOne(ctx, r, tools, cfg, plan)
			if err != nil {
				res.Status = StatusFailed
				res.Note = err.Error()
			} else {
				res.Status = StatusOK
				res.Note = note
			}
		}
		res.Dur = time.Since(start)
		s.Results = append(s.Results, res)
		printResult(w, res)
	}
	return s
}

func printResult(w io.Writer, res Result) {
	line := fmt.Sprintf("%s -> %s  %s", res.Plan.In, res.Plan.Out, res.Status)
	if res.Status == StatusOK {
		line += fmt.Sprintf(" %.1fs", res.Dur.Seconds())
	}
	if res.Note != "" {
		line += "  " + res.Note
	}
	fmt.Fprintln(w, line)
}

func runOne(ctx context.Context, r calibre.Runner, tools calibre.Tools, cfg Config, plan Plan) (string, error) {
	tmp, err := os.MkdirTemp("", "booksmith-*")
	if err != nil {
		return "", err
	}
	if !cfg.KeepTemp {
		defer os.RemoveAll(tmp)
	}
	isRecover := cfg.Mode == Recover || cfg.Mode == ConvertRecover
	if isRecover {
		if _, err := cover.WriteScript(tmp); err != nil {
			return "", err
		}
	}
	for _, st := range steps(cfg, plan, tmp) {
		if st.copyFrom != "" {
			if err := copyFile(st.copyFrom, st.copyTo); err != nil {
				return "", err
			}
			continue
		}
		path := tools.EbookConvert
		if st.tool == "calibre-debug" {
			path = tools.CalibreDebug
		}
		if err := r.Run(ctx, path, st.args...); err != nil {
			return "", err
		}
	}
	if err := moveFile(finalPath(cfg, tmp), plan.Out); err != nil {
		return "", err
	}
	if cfg.KeepTemp {
		return "temp kept: " + tmp, nil
	}
	return "", nil
}

type step struct {
	tool     string // "ebook-convert" or "calibre-debug"; empty for an internal copy
	args     []string
	copyFrom string
	copyTo   string
}

func steps(cfg Config, plan Plan, tmp string) []step {
	final := finalPath(cfg, tmp)
	switch cfg.Mode {
	case Convert:
		return []step{{tool: "ebook-convert", args: []string{plan.In, final}}}
	case Recover, ConvertRecover:
		work := filepath.Join(tmp, "work.epub")
		var out []step
		if batch.IsEpub(plan.In) {
			out = append(out, step{copyFrom: plan.In, copyTo: work})
		} else {
			out = append(out, step{tool: "ebook-convert", args: []string{plan.In, work}})
		}
		out = append(out, step{tool: "calibre-debug", args: []string{
			"-e", filepath.Join(tmp, "booksmith-strip-cover.py"), "--", work,
		}})
		args := []string{work, final}
		if cfg.NoSVGCover {
			args = append(args, "--no-svg-cover")
		}
		return append(out, step{tool: "ebook-convert", args: args})
	default:
		return nil
	}
}

func finalPath(cfg Config, tmp string) string {
	if cfg.Mode == Convert {
		return filepath.Join(tmp, "out."+cfg.targetExt())
	}
	return filepath.Join(tmp, "final.epub")
}

// dryRunLine renders the calibre commands a real run would execute.
func dryRunLine(cfg Config, plan Plan, tools calibre.Tools) string {
	tmp := filepath.Join(os.TempDir(), "booksmith-XXXX")
	var lines []string
	for _, st := range steps(cfg, plan, tmp) {
		if st.copyFrom != "" {
			lines = append(lines, fmt.Sprintf("copy %s %s", st.copyFrom, st.copyTo))
			continue
		}
		path := tools.EbookConvert
		if st.tool == "calibre-debug" {
			path = tools.CalibreDebug
		}
		lines = append(lines, quoteCommand(append([]string{path}, st.args...)))
	}
	return strings.Join(lines, " && ")
}

func quoteCommand(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t\"") {
			quoted[i] = `"` + a + `"`
		} else {
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

func moveFile(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	// Cross-device: copy then remove.
	if err := copyFile(from, to); err != nil {
		os.Remove(to)
		return err
	}
	return os.Remove(from)
}
