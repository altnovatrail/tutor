package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/board"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

func TestBoardCommand(t *testing.T) {
	keys := map[string]string{anthropicKeyVar: "a-key", openAIKeyVar: "o-key"}
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		serveErr   error
		wantCode   int
		wantServed bool
		wantOut    string
	}{
		{"both keys", nil, keys, nil, exitOK, true, "http://127.0.0.1:8080"},
		{"one key", []string{"-addr", "localhost:9000"}, map[string]string{openAIKeyVar: "o-key"}, nil, exitOK, true, "Claude is left out"},
		{"no keys", nil, nil, nil, exitUsage, false, "no model to ask"},
		{"a public address", []string{"-addr", "0.0.0.0:8080"}, keys, nil, exitUsage, false, "not a local address"},
		{"a host name", []string{"-addr", "example.com:8080"}, keys, nil, exitUsage, false, "not a local address"},
		{"no cap", []string{"-max-usd", "0"}, keys, nil, exitUsage, false, "-max-usd must be above 0"},
		{"a stray argument", []string{"extra"}, keys, nil, exitUsage, false, "unexpected arguments"},
		{"the port is taken", nil, keys, errors.New("address already in use"), exitFail, true, "address already in use"},
		// Handwriting goes to samples alone, never to the folder agents work with.
		{"the synthetic folder", []string{"-dir", "../../synthetic/board"}, keys, nil, exitUsage, false, "outside its samples folder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			served := false
			serve := func(s *http.Server) error {
				served = true
				if s.Handler == nil {
					t.Error("the server has no handler")
				}
				return tt.serveErr
			}
			args := append([]string{"-dir", t.TempDir()}, tt.args...)
			got := runBoard(args, &stdout, &stderr, func(k string) string { return tt.env[k] }, serve)
			if got != tt.wantCode || served != tt.wantServed {
				t.Errorf("code %d, served %v; want %d, %v (stderr %q)", got, served, tt.wantCode, tt.wantServed, stderr.String())
			}
			if out := stdout.String() + stderr.String(); !strings.Contains(out, tt.wantOut) {
				t.Errorf("output %q, want it to hold %q", out, tt.wantOut)
			}
		})
	}
}

// The board offers every current Claude model at every effort level, ticking Claude Opus 5.5 at
// high as the board asked before, and GPT-6 Astra at high alone, unticked. Haiku 5.5 and Sonnet
// 5.5 may also be asked without thinking, at high and below; the others always think.
func TestBoardOffers(t *testing.T) {
	var handler http.Handler
	serve := func(s *http.Server) error { handler = s.Handler; return nil }
	env := map[string]string{anthropicKeyVar: "a-key", openAIKeyVar: "o-key"}
	var stdout, stderr bytes.Buffer
	if code := runBoard([]string{"-dir", t.TempDir()}, &stdout, &stderr, func(k string) string { return env[k] }, serve); code != exitOK {
		t.Fatalf("code %d, stderr %q", code, stderr.String())
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/models", nil)
	req.Host = "127.0.0.1:8080"
	handler.ServeHTTP(rec, req)
	var offers []board.Offer
	if err := json.Unmarshal(rec.Body.Bytes(), &offers); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range offers {
		got = append(got, fmt.Sprintf("%s %s %s %v [%s]", o.Model, strings.Join(o.Efforts, ","), o.Effort, o.Ticked, strings.Join(o.NoThinking, ",")))
	}
	want := []string{
		"Claude Haiku 5.5 low,medium,high,xhigh,max high false [low,medium,high]",
		"Claude Sonnet 5.5 low,medium,high,xhigh,max high false [low,medium,high]",
		"Claude Opus 5.5 low,medium,high,xhigh,max high true []",
		"Claude Fable 5.1 low,medium,high,xhigh,max high false []",
		"GPT-6 Astra high high false []",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("offers:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The record mode needs no key and takes no spending cap, and the flags of each mode are its own.
// It does not start with a bank or a plan it cannot read, check or match, and it guards its
// directory before it makes it.
func TestBoardRecordCommand(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	brokenPlan := write("broken-plan.json", []byte(`{"format": "tutor-sheet-plan", "version": 0, "sheets": []}`))
	synthBank, err := sample.Marshal(synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	otherBank := write("other-bank.json", synthBank)
	blocker := write("blocker", nil)
	files := []string{"-bank", measurementBank, "-plan", measurementPlan}
	keys := map[string]string{anthropicKeyVar: "a-key"}
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantCode   int
		wantServed bool
		wantOut    string
	}{
		{"the record mode", append([]string{"-record", "-dir", t.TempDir()}, files...), nil, exitOK, true, "The record mode of the board is at http://127.0.0.1:8080"},
		{"a spending cap", append([]string{"-record", "-dir", t.TempDir(), "-max-usd", "3"}, files...), nil, exitUsage, false, "-max-usd is for the check board"},
		{"a bank on the check board", []string{"-dir", t.TempDir(), "-bank", measurementBank}, keys, exitUsage, false, "-bank and -plan are for the record mode"},
		{"a plan on the check board", []string{"-dir", t.TempDir(), "-plan", measurementPlan}, keys, exitUsage, false, "-bank and -plan are for the record mode"},
		{"a broken plan", []string{"-record", "-dir", t.TempDir(), "-bank", measurementBank, "-plan", brokenPlan}, nil, exitFail, false, "invalid plan: sheets: are missing"},
		{"no bank", []string{"-record", "-dir", t.TempDir(), "-bank", filepath.Join(dir, "nowhere.json"), "-plan", measurementPlan}, nil, exitFail, false, "nowhere.json: open"},
		{"a plan of tasks not in the bank", []string{"-record", "-dir", t.TempDir(), "-bank", otherBank, "-plan", measurementPlan}, nil, exitFail, false, `sheets[0].task: "posts" is not in the bank`},
		{"a public address", append([]string{"-record", "-dir", t.TempDir(), "-addr", "0.0.0.0:8080"}, files...), nil, exitUsage, false, "not a local address"},
		{"an empty directory", append([]string{"-record", "-dir", ""}, files...), nil, exitUsage, false, "-dir is empty"},
		{"a directory that cannot be made", append([]string{"-record", "-dir", filepath.Join(blocker, "sheets")}, files...), nil, exitFail, false, "tutor-eval board: mkdir "},
		// The tests run in cmd/tutor-eval, where samples/sheets is not the samples folder at the root.
		{"the default directory", append([]string{"-record"}, files...), nil, exitUsage, false, "cmd/tutor-eval/samples/sheets is in the git work tree"},
		{"the synthetic folder", append([]string{"-record", "-dir", "../../synthetic/sheets"}, files...), nil, exitUsage, false, "outside its samples folder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var handler http.Handler
			serve := func(s *http.Server) error { handler = s.Handler; return nil }
			got := runBoard(tt.args, &stdout, &stderr, func(k string) string { return tt.env[k] }, serve)
			if got != tt.wantCode || (handler != nil) != tt.wantServed {
				t.Errorf("code %d, served %v; want %d, %v (stderr %q)", got, handler != nil, tt.wantCode, tt.wantServed, stderr.String())
			}
			if out := stdout.String() + stderr.String(); !strings.Contains(out, tt.wantOut) {
				t.Errorf("output %q, want it to hold %q", out, tt.wantOut)
			}
			if handler == nil {
				return
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/record/plan", nil)
			req.Host = "127.0.0.1:8080"
			handler.ServeHTTP(rec, req)
			var view board.PlanView
			if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil || len(view.Sheets) != 40 {
				t.Errorf("GET /api/record/plan = %d %s, want the plan's 40 sheets", rec.Code, rec.Body)
			}
		})
	}
	if _, err := os.Stat("samples"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused default directory was made: %v", err)
	}
}

func TestCheckIgnored(t *testing.T) {
	// The tests run in cmd/tutor-eval, two levels below the root of tutor, whose .gitignore ignores
	// the samples and synthetic folders at the root only.
	if err := checkIgnored("../../samples/board", samplesFolder); err != nil {
		t.Errorf("the samples folder at the root: %v, want it taken", err)
	}
	if err := checkIgnored("samples/board", samplesFolder); err == nil || !strings.Contains(err.Error(), "outside its samples folder") {
		t.Errorf("a samples folder below the root = %v, want it refused", err)
	}
	if err := checkIgnored(t.TempDir(), samplesFolder); err != nil {
		t.Errorf("a folder outside any work tree: %v, want it taken", err)
	}
	// A folder whose name only starts like the samples folder is not in it.
	if err := checkIgnored("../../samples-copy/board", samplesFolder); err == nil {
		t.Error("a folder beside the samples folder was taken")
	}
	if err := checkIgnored("../../samples", samplesFolder); err != nil {
		t.Errorf("the samples folder itself: %v, want it taken", err)
	}

	// The board writes handwriting, which goes to samples alone; what code draws may go to either.
	if err := checkIgnored("../../synthetic/board", samplesFolder); err == nil || !strings.Contains(err.Error(), "outside its samples folder") {
		t.Errorf("the synthetic folder for the board = %v, want it refused", err)
	}
	for _, dir := range []string{"../../synthetic/lines", "../../synthetic", "../../samples/sheets"} {
		if err := checkIgnored(dir, samplesFolder, syntheticFolder); err != nil {
			t.Errorf("%s for the sheets drawn: %v, want it taken", dir, err)
		}
	}
	for _, dir := range []string{"synthetic/lines", "../../synthetic-copy/lines", "../../eval"} {
		if err := checkIgnored(dir, samplesFolder, syntheticFolder); err == nil || !strings.Contains(err.Error(), "outside its samples and synthetic folders") {
			t.Errorf("%s for the sheets drawn = %v, want it refused", dir, err)
		}
	}
}

// The folders checkIgnored takes are the ones tutor's .gitignore ignores, at the root alone.
func TestIgnoredFolders(t *testing.T) {
	data, err := os.ReadFile("../../.gitignore")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	for _, folder := range []string{samplesFolder, syntheticFolder} {
		if !slices.Contains(lines, "/"+folder+"/") {
			t.Errorf(".gitignore has no line /%s/", folder)
		}
	}
}
