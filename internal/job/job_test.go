package job

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"booksmith/internal/calibre"
)

var tools = calibre.Tools{EbookConvert: "/calibre/ebook-convert", CalibreDebug: "/calibre/calibre-debug"}

type call struct {
	tool string
	args []string
}

type fakeRunner struct {
	calls []call
	err   error
}

func (f *fakeRunner) Run(ctx context.Context, tool string, args ...string) error {
	f.calls = append(f.calls, call{tool: tool, args: args})
	if f.err != nil {
		return f.err
	}
	if strings.Contains(filepath.Base(tool), "ebook-convert") && len(args) >= 2 {
		return os.WriteFile(args[1], []byte("converted"), 0o644)
	}
	return nil
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s should exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist", path)
	}
}

func TestBuildConvertNaming(t *testing.T) {
	dir := t.TempDir()
	plans := Build(Config{Mode: Convert}, []string{filepath.Join(dir, "a.v2.mobi")})
	if len(plans) != 1 {
		t.Fatalf("got %d plans", len(plans))
	}
	if want := filepath.Join(dir, "a.v2.epub"); plans[0].Out != want {
		t.Errorf("Out = %q, want %q", plans[0].Out, want)
	}
	if plans[0].Skip != "" {
		t.Errorf("unexpected skip: %q", plans[0].Skip)
	}
}

func TestBuildConvertTargetFormat(t *testing.T) {
	dir := t.TempDir()
	plans := Build(Config{Mode: Convert, To: "AZW3"}, []string{filepath.Join(dir, "a.mobi")})
	if want := filepath.Join(dir, "a.azw3"); plans[0].Out != want {
		t.Errorf("Out = %q, want %q", plans[0].Out, want)
	}
}

func TestBuildConvertSkipsOverwritingInput(t *testing.T) {
	dir := t.TempDir()
	plans := Build(Config{Mode: Convert}, []string{filepath.Join(dir, "a.epub")})
	if plans[0].Skip != "would overwrite the input" {
		t.Errorf("Skip = %q", plans[0].Skip)
	}
}

func TestBuildSkipsExistingOutput(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.epub"), "already here")
	plans := Build(Config{Mode: Convert}, []string{filepath.Join(dir, "a.mobi")})
	if plans[0].Skip != "exists" {
		t.Errorf("Skip = %q", plans[0].Skip)
	}
}

func TestBuildOutDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	plans := Build(Config{Mode: Convert, OutDir: out}, []string{filepath.Join(dir, "books", "a.mobi")})
	if want := filepath.Join(out, "a.epub"); plans[0].Out != want {
		t.Errorf("Out = %q, want %q", plans[0].Out, want)
	}
}

func TestBuildRecover(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "a.epub")
	mobi := filepath.Join(dir, "b.mobi")
	plans := Build(Config{Mode: Recover}, []string{epub, mobi})
	if want := filepath.Join(dir, "a.recover.epub"); plans[0].Out != want {
		t.Errorf("Out = %q, want %q", plans[0].Out, want)
	}
	if plans[0].Skip != "" {
		t.Errorf("epub should not be skipped: %q", plans[0].Skip)
	}
	if plans[1].Skip != "not an epub (use convert-recover)" {
		t.Errorf("Skip = %q", plans[1].Skip)
	}
}

func TestBuildConvertRecover(t *testing.T) {
	dir := t.TempDir()
	mobi := filepath.Join(dir, "a.mobi")
	epub := filepath.Join(dir, "b.epub")
	plans := Build(Config{Mode: ConvertRecover}, []string{mobi, epub})
	if want := filepath.Join(dir, "a.epub"); plans[0].Out != want {
		t.Errorf("Out = %q, want %q", plans[0].Out, want)
	}
	if plans[0].Skip != "" {
		t.Errorf("mobi should not be skipped: %q", plans[0].Skip)
	}
	if plans[1].Skip != "epub input (use recover)" {
		t.Errorf("Skip = %q", plans[1].Skip)
	}
}

func TestExecuteConvert(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "original")
	cfg := Config{Mode: Convert}
	r := &fakeRunner{}
	var out bytes.Buffer

	s := Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &out)

	if s.OK() != 1 || s.Failed() != 0 || s.ExitCode() != 0 {
		t.Fatalf("summary = %+v", s)
	}
	if len(r.calls) != 1 {
		t.Fatalf("got %d calibre calls, want 1: %+v", len(r.calls), r.calls)
	}
	if got := r.calls[0].args[0]; got != in {
		t.Errorf("input arg = %q, want %q", got, in)
	}
	if got := filepath.Base(r.calls[0].args[1]); got != "out.epub" {
		t.Errorf("output arg = %q, want the temp epub", got)
	}
	mustExist(t, filepath.Join(dir, "a.epub"))
	if data, err := os.ReadFile(in); err != nil || string(data) != "original" {
		t.Errorf("input was modified: %q %v", data, err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Errorf("output = %q", out.String())
	}
}

func TestExecuteRecoverFromEpub(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.epub")
	write(t, in, "epub bytes")
	cfg := Config{Mode: Recover}
	r := &fakeRunner{}

	Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &bytes.Buffer{})

	if len(r.calls) != 2 {
		t.Fatalf("got %d calls, want 2 (strip, convert): %+v", len(r.calls), r.calls)
	}
	strip := r.calls[0]
	if strip.tool != tools.CalibreDebug {
		t.Errorf("first call tool = %q", strip.tool)
	}
	if strip.args[0] != "-e" || strip.args[2] != "--" {
		t.Errorf("strip args = %v", strip.args)
	}
	if filepath.Base(strip.args[1]) != "booksmith-strip-cover.py" {
		t.Errorf("script = %q", strip.args[1])
	}
	if filepath.Base(strip.args[3]) != "work.epub" {
		t.Errorf("work file = %q", strip.args[3])
	}
	convert := r.calls[1]
	if filepath.Base(convert.args[0]) != "work.epub" || filepath.Base(convert.args[1]) != "final.epub" {
		t.Errorf("convert args = %v", convert.args)
	}
	if len(convert.args) != 2 {
		t.Errorf("unexpected extra args: %v", convert.args[2:])
	}
	mustExist(t, filepath.Join(dir, "a.recover.epub"))
}

func TestExecuteConvertRecoverFromMobi(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "mobi bytes")
	cfg := Config{Mode: ConvertRecover}
	r := &fakeRunner{}

	Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &bytes.Buffer{})

	if len(r.calls) != 3 {
		t.Fatalf("got %d calls, want 3 (convert, strip, convert): %+v", len(r.calls), r.calls)
	}
	if r.calls[0].args[0] != in || filepath.Base(r.calls[0].args[1]) != "work.epub" {
		t.Errorf("first call args = %v", r.calls[0].args)
	}
	mustExist(t, filepath.Join(dir, "a.epub"))
}

func TestExecuteNoSVGCoverIsPassed(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.epub")
	write(t, in, "epub bytes")
	cfg := Config{Mode: Recover, NoSVGCover: true}
	r := &fakeRunner{}

	Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &bytes.Buffer{})

	last := r.calls[len(r.calls)-1].args
	if last[len(last)-1] != "--no-svg-cover" {
		t.Errorf("args = %v, want a trailing --no-svg-cover", last)
	}
}

func TestExecuteFailureKeepsGoingAndKeepsInputs(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.mobi")
	second := filepath.Join(dir, "b.mobi")
	write(t, first, "one")
	write(t, second, "two")
	cfg := Config{Mode: Convert}
	r := &fakeRunner{err: &calibre.CmdError{Detail: "boom", Err: errors.New("exit status 1")}}

	s := Execute(context.Background(), r, tools, cfg, Build(cfg, []string{first, second}), &bytes.Buffer{})

	if s.Failed() != 2 || s.ExitCode() != 1 {
		t.Fatalf("summary = %+v", s)
	}
	if len(r.calls) != 2 {
		t.Errorf("both inputs should have been attempted, got %d calls", len(r.calls))
	}
	mustNotExist(t, filepath.Join(dir, "a.epub"))
	mustNotExist(t, filepath.Join(dir, "b.epub"))
	mustExist(t, first)
}

func TestExecuteFailureReportedOnTheLine(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "one")
	cfg := Config{Mode: Convert}
	r := &fakeRunner{err: &calibre.CmdError{Detail: "Cannot detect cover image"}}
	var out bytes.Buffer

	Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &out)

	if got := out.String(); !strings.Contains(got, "fail") || !strings.Contains(got, "calibre: Cannot detect cover image") {
		t.Errorf("output = %q", got)
	}
}

func TestExecuteSkipsWithoutRunningCalibre(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "one")
	write(t, filepath.Join(dir, "a.epub"), "existing")
	cfg := Config{Mode: Convert}
	r := &fakeRunner{}

	s := Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &bytes.Buffer{})

	if len(r.calls) != 0 {
		t.Errorf("calibre should not have run: %+v", r.calls)
	}
	if s.Skipped() != 1 || s.ExitCode() != 0 {
		t.Errorf("summary = %+v", s)
	}
}

func TestExecuteDryRunPrintsCommandsOnly(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "one")
	cfg := Config{Mode: ConvertRecover, DryRun: true}
	r := &fakeRunner{}
	var out bytes.Buffer

	s := Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &out)

	if len(r.calls) != 0 {
		t.Errorf("dry run must not execute anything: %+v", r.calls)
	}
	if s.OK() != 0 || s.count(StatusDryRun) != 1 {
		t.Errorf("summary = %+v", s)
	}
	got := out.String()
	if !strings.Contains(got, "ebook-convert") || !strings.Contains(got, "calibre-debug -e") {
		t.Errorf("dry-run output should show both tools: %q", got)
	}
	mustNotExist(t, filepath.Join(dir, "a.epub"))
}

func TestExecuteKeepTempMentionsDirectory(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	write(t, in, "one")
	cfg := Config{Mode: Convert, KeepTemp: true}
	r := &fakeRunner{}

	s := Execute(context.Background(), r, tools, cfg, Build(cfg, []string{in}), &bytes.Buffer{})

	note := s.Results[0].Note
	if !strings.HasPrefix(note, "temp kept: ") {
		t.Fatalf("note = %q", note)
	}
	tmp := strings.TrimPrefix(note, "temp kept: ")
	t.Cleanup(func() { os.RemoveAll(tmp) })
	mustExist(t, tmp)
}

func TestSummaryString(t *testing.T) {
	s := Summary{Results: []Result{
		{Status: StatusOK}, {Status: StatusOK}, {Status: StatusSkipped}, {Status: StatusFailed},
	}}
	if got, want := s.String(), "2 ok, 1 skipped, 1 failed"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := (Summary{}).String(), "nothing to do"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
