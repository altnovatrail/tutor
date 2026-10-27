package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/board"
	"github.com/altnovatrail/tutor/provider/claude"
	"github.com/altnovatrail/tutor/provider/gpt"
)

// The variables that hold the keys for eval (R24 spec; Т-28). They are not ANTHROPIC_API_KEY and
// OPENAI_API_KEY on purpose: with ANTHROPIC_API_KEY set, Claude Code in the same container would
// bill its own work to the eval key.
const (
	anthropicKeyVar = "TUTOR_ANTHROPIC_API_KEY"
	openAIKeyVar    = "TUTOR_OPENAI_API_KEY"
)

// The page starts as the board asked before it offered a choice: Claude Opus 5.5 at high effort
// (R19).
const (
	startModel  = "claude-opus-5-5"
	startEffort = "high"
)

func serveHTTP(s *http.Server) error { return s.ListenAndServe() }

// choices are the models the board offers: every current Claude model at every effort level with
// the Anthropic key — without thinking, too, where its thinking can be turned off — and GPT-6
// Astra at high alone with the OpenAI key, unticked, since GPT is not used on the board (R20).
func choices(getenv func(string) string, stderr io.Writer) []board.Choice {
	var offered []board.Choice
	if key := getenv(anthropicKeyVar); key != "" {
		for _, m := range claude.Models {
			offered = append(offered, board.Choice{Model: claude.New(key, m), Efforts: claude.Efforts, Effort: startEffort, Ticked: m.ID == startModel,
				NoThinking: m.NoThinkingEfforts()})
		}
	} else {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %s is not set: Claude is left out\n", anthropicKeyVar)
	}
	if key := getenv(openAIKeyVar); key != "" {
		offered = append(offered, board.Choice{Model: gpt.New(key), Efforts: gpt.Efforts, Effort: gpt.Efforts[0]})
	} else {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %s is not set: GPT is left out\n", openAIKeyVar)
	}
	return offered
}

// The folders at the root of tutor that its .gitignore ignores. samples holds the author's
// handwriting, and no agent opens it (R05). synthetic holds what code draws and the runs on it,
// with no handwriting, and agents work with it (R28).
const (
	samplesFolder   = "samples"
	syntheticFolder = "synthetic"
)

// checkIgnored refuses a directory for what a command writes or reads that lies in a git work tree
// outside the folders named, at the root of the tree. The board and its record mode name samples
// alone: the author's handwriting goes nowhere else, neither one `git add` away from the repository
// nor into the folder agents work with. The commands of the sheets — draw, sheets, split and run —
// name synthetic too. It runs before the directory is created, and the default directories work
// from the root alone. A directory outside any work tree has nothing to guard.
func checkIgnored(dir, folder string, more ...string) error {
	abs, root, in, err := placeOf(dir)
	folders := append([]string{folder}, more...)
	if err != nil || root == "" || slices.Contains(folders, in) {
		return err
	}
	where := "its " + folder + " folder"
	if len(more) > 0 {
		where = "its " + strings.Join(folders, " and ") + " folders"
	}
	return fmt.Errorf("%s is in the git work tree %s outside %s: run tutor-eval from the root of tutor, or give it a folder in %s", abs, root, where, strings.Join(folders, " or "))
}

// placeOf finds where a directory lies: its absolute path, the root of the git work tree it is in,
// or "" outside any, and the ignored folder at that root it is in, or "" if neither.
func placeOf(dir string) (abs, root, folder string, err error) {
	if abs, err = filepath.Abs(dir); err != nil {
		return "", "", "", err
	}
	for root = abs; ; {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			for _, f := range []string{samplesFolder, syntheticFolder} {
				if within(filepath.Join(root, f), abs) {
					return abs, root, f, nil
				}
			}
			return abs, root, "", nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return abs, "", "", nil
		}
		root = parent
	}
}

// within reports whether the absolute path is base itself or lies inside it.
func within(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// The directories each mode of the board writes to by default, in the samples folder at the root
// of tutor, which git ignores (R05).
const (
	boardDir  = samplesFolder + "/board"
	sheetsDir = samplesFolder + "/sheets"
)

// boardFlags are the flags of tutor-eval board, and which of them the command line set.
type boardFlags struct {
	addr, dir, bank, plan string
	maxUSD                float64
	record                bool
	set                   map[string]bool
}

// runBoard starts the board on a local address: the check board, or with -record its record mode.
// getenv and serve are the process's, so that a test can drive the command without keys or a port.
func runBoard(args []string, stdout, stderr io.Writer, getenv func(string) string, serve func(*http.Server) error) int {
	flags := flag.NewFlagSet("tutor-eval board", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var f boardFlags
	flags.StringVar(&f.addr, "addr", "127.0.0.1:8080", "the local address to serve the board on")
	flags.StringVar(&f.dir, "dir", "", "where the board writes, out of git: the pictures and the log of marks, "+boardDir+" by default, or with -record the sheets, "+sheetsDir)
	flags.Float64Var(&f.maxUSD, "max-usd", 5, "stop asking the models once the calls of this run cost this many US dollars; not with -record")
	flags.BoolVar(&f.record, "record", false, "the record mode: write the sheets of the plan and save each as a sample, asking no model")
	flags.StringVar(&f.bank, "bank", "eval/bank.json", "with -record: the bank of tasks")
	flags.StringVar(&f.plan, "plan", "eval/sheets.json", "with -record: the sheet plan")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	f.set = given(flags)
	if f.set["dir"] && f.dir == "" {
		_, _ = fmt.Fprintln(stderr, "tutor-eval board: -dir is empty: name the directory, or leave the flag out for the default")
		return exitUsage
	}
	host, _, err := net.SplitHostPort(f.addr)
	if ip := net.ParseIP(host); err != nil || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %q is not a local address: the board serves only this machine, and the check board spends the eval keys\n", f.addr)
		return exitUsage
	}

	var handler http.Handler
	var ready string
	var code int
	if f.record {
		handler, ready, code = recordMode(f, stderr)
	} else {
		handler, ready, code = checkBoard(f, getenv, stderr)
	}
	if code != exitOK {
		return code
	}
	if _, err := fmt.Fprintln(stdout, ready); err != nil {
		return exitFail
	}
	srv := &http.Server{Addr: f.addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	if err := serve(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return exitFail
	}
	return exitOK
}

// checkBoard makes the check board: the models whose keys are set, the spending cap, and the
// directory for the pictures and the log. It returns the handler, the line that says the board is
// ready, and the exit code when it cannot start.
func checkBoard(f boardFlags, getenv func(string) string, stderr io.Writer) (http.Handler, string, int) {
	if f.set["bank"] || f.set["plan"] {
		_, _ = fmt.Fprintln(stderr, "tutor-eval board: -bank and -plan are for the record mode, -record")
		return nil, "", exitUsage
	}
	if f.maxUSD <= 0 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval board: -max-usd must be above 0")
		return nil, "", exitUsage
	}
	offered := choices(getenv, stderr)
	if len(offered) == 0 {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: no model to ask: set %s or %s\n", anthropicKeyVar, openAIKeyVar)
		return nil, "", exitUsage
	}
	dir := cmp.Or(f.dir, boardDir)
	if err := checkIgnored(dir, samplesFolder); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return nil, "", exitUsage
	}
	b, err := board.New(offered, dir, f.maxUSD)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return nil, "", exitFail
	}
	return b.Handler(f.addr), fmt.Sprintf("The board is at http://%s — open it in a browser. Pictures and marks go to %s.", f.addr, dir), exitOK
}

// recordMode makes the record mode: the bank and the plan, read and checked, and the directory for
// the sheets. It asks no model, so it needs no key and no spending cap. It returns the handler,
// the line that says the board is ready, and the exit code when it cannot start.
func recordMode(f boardFlags, stderr io.Writer) (http.Handler, string, int) {
	if f.set["max-usd"] {
		_, _ = fmt.Fprintln(stderr, "tutor-eval board: -max-usd is for the check board: the record mode asks no model")
		return nil, "", exitUsage
	}
	dir := cmp.Or(f.dir, sheetsDir)
	if err := checkIgnored(dir, samplesFolder); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return nil, "", exitUsage
	}
	bank, plan, err := readPlan(f.bank, f.plan)
	if err == nil {
		err = linesOnly(f.plan, plan, "the record mode writes")
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return nil, "", exitFail
	}
	rc, err := board.NewRecorder(bank, plan, dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return nil, "", exitFail
	}
	return rc.Handler(f.addr), fmt.Sprintf("The record mode of the board is at http://%s — open it in a browser. Sheets go to %s.", f.addr, dir), exitOK
}

// readFile reads a file of the format with its reader.
func readFile[T any](path string, read func(io.Reader) (T, error)) (T, error) {
	f, err := os.Open(path)
	if err != nil {
		var none T
		return none, err
	}
	defer func() { _ = f.Close() }()
	return read(f)
}
