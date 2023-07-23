package main

import (
	"context"
	"os/exec"
	"time"
)

// this new type is needed to handle steps that might hang or take too long
type timeoutStep struct {
	step
	timeout time.Duration
}

func newTimeoutStep(name, exe, message, proj string, args []string, timeout time.Duration) timeoutStep {
	s := timeoutStep{}
	s.step = newStep(name, exe, message, proj, args)
	s.timeout = timeout
	if s.timeout == 0 {
		s.timeout = 30 * time.Second
	}
	return s
}

// we will use this variable to
// add a mock function and execute it when testing
var command = exec.CommandContext

func (s timeoutStep) execute() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	// run the cancel function to free up resources when the context is no longer required
	defer cancel()
	// this is going to execute the command using the context
	// the WithTimeout defined a timeout for the current context
	// if the timeout runs out the CommandContext function will kill the current
	// running process
	cmd := command(ctx, s.exe, s.args...)
	cmd.Dir = s.proj
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", &stepErr{
				step:  s.name,
				msg:   "failed timeout",
				cause: context.DeadlineExceeded,
			}
		}

		return "", &stepErr{
			step:  s.name,
			msg:   "failed to execute",
			cause: err,
		}
	}

	return s.message, nil
}
