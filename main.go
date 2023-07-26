package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type executer interface {
	execute() (string, error)
}

func main() {
	proj := flag.String("p", "", "Project directory")
	flag.Parse()

	if err := run(*proj, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(proj string, out io.Writer) error {
	if proj == "" {
		return fmt.Errorf("project directory is required %w", ErrValidation)
	}

	pipeline := make([]executer, 6)
	// handle at least one signal concurrently in case any signal is received
	sig := make(chan os.Signal, 1)
	errCh := make(chan error)
	done := make(chan struct{})
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	pipeline[0] = newStep(
		"go build",
		"go",
		"Go Build: SUCCESS",
		proj,
		[]string{"build", ".", "errors"}, // we are adding the errors package to not generate a binary
	)
	pipeline[1] = newStep(
		"go test",
		"go",
		"Go Test: SUCCESS",
		proj,
		[]string{"test", "-v"},
	)
	pipeline[2] = newExceptionStep(
		"go format",
		"gofmt",
		"Go Format: SUCCESS",
		proj,
		[]string{"-l", "./"},
	)
	pipeline[3] = newExceptionStep(
		"lint",
		"golangci-lint",
		"Go Lint: SUCCESS",
		proj,
		[]string{"run", "."},
	)
	pipeline[4] = newExceptionStep(
		"cyclo",
		"gocyclo",
		"Go Cyclo: SUCCESS",
		proj,
		[]string{"-over", "10", "."},
	)
	pipeline[5] = newTimeoutStep(
		"git push",
		"git",
		"Git Push: SUCCESS",
		proj,
		[]string{"push", "origin", "master"},
		10*time.Second,
	)

	go func() {
		for _, s := range pipeline {
			msg, err := s.execute()
			if err != nil {
				errCh <- err
			}

			_, err = fmt.Fprintln(out, msg)
			if err != nil {
				errCh <- err
			}
		}
		close(done)
	}()

	for {
		select {
		case rec := <-sig:
			signal.Stop(sig)
			return fmt.Errorf("%s: Exiting: %w", rec, ErrSignal)
		case err := <-errCh:
			return err
		case <-done:
			return nil
		}
	}
}
