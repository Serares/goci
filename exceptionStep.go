package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// this is a type of inheritance in go
type exceptionStep struct {
	step // we can access the fields of this type directly; example: exceptionStep.exe
}

// embedding one type into another
// it's a reusability pattern
func newExceptionStep(name, exe, message, proj string, args []string) exceptionStep {
	s := exceptionStep{}
	s.step = newStep(name, exe, message, proj, args)
	return s
}

// add a new method on the step struct to handle
// stdout from commands
// instead of adding the logic in the step.execute() func
// create a new struct that embeds the original step{} and overrides it's execute func
// this method is somehow an override to the original step.execute()
func (e exceptionStep) execute() (string, error) {
	cmd := exec.Command(e.exe, e.args...)
	// create a buffer to capture the output of the command
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Dir = e.proj
	if err := cmd.Run(); err != nil {
		return "", &stepErr{
			step:  e.name,
			msg:   "failed to execute",
			cause: err,
		}
	}

	if out.Len() > 0 {
		return "", &stepErr{
			step:  e.name,
			msg:   fmt.Sprintf("invalid format %s", out.String()),
			cause: nil,
		}
	}

	return e.message, nil
}
