package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

type executer interface {
	execute() (string, error)
}

type config struct {
	branch       string
	project      string
	steps        []executer
	pipelinePath string
}

func main() {
	proj := flag.String("p", "", "Project directory")
	gitBranch := flag.String("branch", "", "git branch to push to")
	pipelinePath := flag.String("pipeline", "", "path of the pipeline yaml file that holds the steps")
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Provide the flags as in the following list:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	cfg := config{
		branch:       *gitBranch,
		project:      *proj,
		pipelinePath: *pipelinePath,
	}
	steps, err := generateSteps(cfg)
	if err != nil {
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg.steps = steps
	if err := run(cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(config config, out io.Writer) error {
	// handle at least one signal concurrently in case any signal is received
	sig := make(chan os.Signal, 1)
	errCh := make(chan error)
	done := make(chan struct{})
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for _, s := range config.steps {
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
