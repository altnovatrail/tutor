package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
// the Anthropic key, and GPT-6 Astra at high alone with the OpenAI key, unticked, since GPT is not
// used on the board (R20).
func choices(getenv func(string) string, stderr io.Writer) []board.Choice {
	var offered []board.Choice
	if key := getenv(anthropicKeyVar); key != "" {
		for _, m := range claude.Models {
			offered = append(offered, board.Choice{Model: claude.New(key, m), Efforts: claude.Efforts, Effort: startEffort, Ticked: m.ID == startModel})
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

// checkIgnored refuses a directory for the pictures that lies in a git work tree outside the
// samples folder at its root — the one folder tutor's .gitignore ignores (R05) — since the
// author's handwriting would be one `git add` away from the repository. The default -dir works
// from the root alone. A directory outside any work tree has nothing to guard.
func checkIgnored(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for root := abs; ; {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			rel, err := filepath.Rel(filepath.Join(root, "samples"), abs)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s is in the git work tree %s outside its samples folder, which git does not ignore: run the board from the root of tutor, or give -dir a folder in samples", abs, root)
			}
			return nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return nil
		}
		root = parent
	}
}

// runBoard starts the check board on a local address. getenv and serve are the process's, so
// that a test can drive the command without keys or a port.
func runBoard(args []string, stdout, stderr io.Writer, getenv func(string) string, serve func(*http.Server) error) int {
	flags := flag.NewFlagSet("tutor-eval board", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:8080", "the local address to serve the board on")
	dir := flags.String("dir", "samples/board", "where the pictures and the log of marks go; keep it out of git")
	maxUSD := flags.Float64("max-usd", 5, "stop asking the models once the calls of this run cost this many US dollars")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: unexpected arguments: %q\n", flags.Args())
		return exitUsage
	}
	host, _, err := net.SplitHostPort(*addr)
	if ip := net.ParseIP(host); err != nil || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %q is not a local address: the board spends the eval keys and serves only this machine\n", *addr)
		return exitUsage
	}
	if *maxUSD <= 0 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval board: -max-usd must be above 0")
		return exitUsage
	}

	offered := choices(getenv, stderr)
	if len(offered) == 0 {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: no model to ask: set %s or %s\n", anthropicKeyVar, openAIKeyVar)
		return exitUsage
	}

	if err := checkIgnored(*dir); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return exitUsage
	}
	b, err := board.New(offered, *dir, *maxUSD)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return exitFail
	}
	if _, err := fmt.Fprintf(stdout, "The board is at http://%s — open it in a browser. Pictures and marks go to %s.\n", *addr, *dir); err != nil {
		return exitFail
	}
	srv := &http.Server{Addr: *addr, Handler: b.Handler(*addr), ReadHeaderTimeout: 10 * time.Second}
	if err := serve(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_, _ = fmt.Fprintf(stderr, "tutor-eval board: %v\n", err)
		return exitFail
	}
	return exitOK
}
