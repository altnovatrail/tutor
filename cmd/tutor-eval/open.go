package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/openview"
)

// runOpen writes the open views of the tasks of the given files — banks, the workbook and the
// essays — into a new file, the one that tutor's release carries for canvas's sample page (T37; R64).
// The file names each bank by the path given, its path in the repository, so it runs at the root
// alone, as just open-tasks runs it. It never writes over a file.
func runOpen(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval open", flag.ContinueOnError)
	flags.SetOutput(stderr)
	file := flags.String("file", "", "the new file of the open views")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	banks := flags.Args()
	if *file == "" || len(banks) == 0 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval open: give -file, the new file to write, and then the files of the tasks")
		return exitUsage
	}
	if !atRoot() {
		_, _ = fmt.Fprintln(stderr, "tutor-eval open: run it from the repository's root, where each file's path is its path in the repository")
		return exitUsage
	}
	inputs := make([]openview.Input, 0, len(banks))
	for _, path := range banks {
		data, err := os.ReadFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval open: %v\n", err)
			return exitFail
		}
		inputs = append(inputs, openview.Input{Path: path, Data: data})
	}
	f, err := openview.Build(inputs)
	var data []byte
	if err == nil {
		data, err = openview.Marshal(f)
	}
	if err == nil {
		err = newfile.Write(*file, data, 0o644)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval open: %v\n", err)
		return exitFail
	}
	tasks := 0
	for _, b := range f.Banks {
		tasks += len(b.Tasks)
	}
	if _, err := fmt.Fprintf(stdout, "%s: ok, open views of %s from %s\n", *file, count(tasks, "task"), count(len(f.Banks), "file")); err != nil {
		return exitFail
	}
	return exitOK
}

// atRoot reports whether the working directory is the root of tutor's repository: its go.mod names the
// module.
func atRoot() bool {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(first) == "module github.com/altnovatrail/tutor"
}
