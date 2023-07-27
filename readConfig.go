package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type configStep struct {
	Name      string   `yaml:"name"`
	Cmd       string   `yaml:"cmd"`
	Message   string   `yaml:"msg"`
	Proj      string   `yaml:"proj,omitempty"`
	Args      []string `yaml:"args"`
	Timeout   int      `yaml:"timeout"`
	Exception bool     `yaml:"hasException"`
	Branch    string   `yaml:"branch"`
}

type configPipeline struct {
	Steps []configStep `yaml:"steps"`
}

func generateSteps(cfg config) ([]executer, error) {
	if cfg.pipelinePath == "" {
		return nil, fmt.Errorf("path not provided for config %v", ErrConfigRead)
	}

	f, err := os.ReadFile(cfg.pipelinePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the yaml file %s, %v", cfg.pipelinePath, ErrConfigRead)
	}
	var unknownStructure configPipeline
	err = yaml.Unmarshal(f, &unknownStructure)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the yaml %v, %v", err, ErrConfigRead)
	}

	var steps []executer = make([]executer, len(unknownStructure.Steps))
	for inx, s := range unknownStructure.Steps {
		if s.Proj == "" && cfg.project == "" {
			return nil, fmt.Errorf("no project path provided for step %s, %v", s.Name, ErrConfigRead)
		}
		sp := populateTheStep(s, cfg.project, cfg.branch)
		steps[inx] = sp
	}

	return steps, nil
}

func populateTheStep(s configStep, projPath, gitBranch string) executer {
	var pPath string = projPath
	var branch string = gitBranch
	if pPath == "" {
		pPath = s.Proj
	}
	if branch == "" {
		branch = s.Branch
	}
	if s.Timeout != 0 {
		// TODO is this the right thing to do?
		if s.Cmd == "git" {
			s.Args = append(s.Args, branch)
		}
		timeout := time.Duration(s.Timeout) * time.Second
		return newTimeoutStep(s.Name, s.Cmd, s.Message, pPath, s.Args, timeout)
	}

	if s.Exception {
		return newExceptionStep(s.Name, s.Cmd, s.Message, pPath, s.Args)
	}

	return newStep(s.Name, s.Cmd, s.Message, pPath, s.Args)
}
