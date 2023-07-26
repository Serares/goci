package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// integration test
func TestRun(t *testing.T) {
	var successMessage string = "Go Build: SUCCESS\nGo Test: SUCCESS\nGo Format: SUCCESS\nGo Lint: SUCCESS\nGit Push: SUCCESS\n"
	var testCases = []struct {
		name     string
		proj     string
		out      string
		expErr   error
		setupGit bool
		mockCmd  func(ctx context.Context, name string, arg ...string) *exec.Cmd
	}{
		{name: "success", proj: "./testdata/tool/",
			out:      successMessage,
			expErr:   nil,
			setupGit: true,
			mockCmd:  nil,
		},
		{name: "successMock", proj: "./testdata/tool/",
			out:      successMessage,
			expErr:   nil,
			setupGit: false,
			mockCmd:  mockCmdContext,
		},
		{name: "fail", proj: "./testdata/toolErr",
			out:      "",
			expErr:   &stepErr{step: "go build"},
			setupGit: false,
			mockCmd:  nil,
		},
		{name: "failFmt", proj: "./testdata/toolFmtErr/",
			out:      "",
			expErr:   &stepErr{step: "go format"},
			setupGit: false,
			mockCmd:  nil,
		},
		{name: "failLint", proj: "./testdata/toolLintErr/",
			out:      "",
			expErr:   &stepErr{step: "lint"},
			setupGit: false,
			mockCmd:  nil,
		},
		{name: "failTimeout", proj: "./testdata/tool",
			out:      "",
			expErr:   context.DeadlineExceeded,
			setupGit: false,
			mockCmd:  mockCmdTimeout,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupGit {
				_, err := exec.LookPath("git")
				if err != nil {
					t.Skip("Git not installed. Skipping test.")
				}
				cleanup := setupGit(t, tc.proj)
				defer cleanup()
			}

			if tc.mockCmd != nil {
				command = tc.mockCmd
			}
			var out bytes.Buffer

			err := run(tc.proj, &out)
			if tc.expErr != nil {
				if err == nil {
					t.Errorf("Expected error: %q. Got 'nil' instead", tc.expErr)
					return
				}
				if !errors.Is(err, tc.expErr) {
					t.Errorf("Expected error: %q. Got %q.", tc.expErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}

			if out.String() != tc.out {
				t.Errorf("Expected output: %q. Got %q", tc.out, out.String())
			}
		})
	}
}

func TestRunKill(t *testing.T) {
	var testCases = []struct {
		name   string
		proj   string
		sig    syscall.Signal
		expErr error
	}{
		{"SIGINT", "./testdata/tool", syscall.SIGINT, ErrSignal},
		{"SIGTERM", "./testdata/tool", syscall.SIGTERM, ErrSignal},
		{"SIGQUIT", "./testdata/tool", syscall.SIGQUIT, nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// override the command variable with timeout mock
			// to give some time to the app to run the signals
			command = mockCmdTimeout
			errCh := make(chan error)
			ignSigCh := make(chan os.Signal, 1)
			expSigCh := make(chan os.Signal, 1)
			signal.Notify(ignSigCh, syscall.SIGQUIT)
			defer signal.Stop(ignSigCh)
			signal.Notify(expSigCh, tc.sig)
			defer signal.Stop(expSigCh)
			go func() {
				errCh <- run(tc.proj, io.Discard)
			}()

			go func() {
				time.Sleep(2 * time.Second)
				if err := syscall.Kill(syscall.Getpid(), tc.sig); err != nil {
					t.Errorf("error trying to kill the process")
				}
			}()

			select {
			case err := <-errCh:
				if err == nil {
					t.Errorf("expected error. Got 'nil' instead.")
					return
				}

				if !errors.Is(err, tc.expErr) {
					t.Errorf("expected error: %q. Got %q", tc.expErr, err)
				}

				select {
				case rec := <-expSigCh:
					if rec != tc.sig {
						t.Errorf("expected signal %q, got %q", tc.sig, rec)
					}
				default:
					t.Errorf("signal not received")
				}
			case <-ignSigCh:
			}
		})
	}
}

func mockCmdContext(ctx context.Context, exe string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess"}

	cs = append(cs, exe)
	cs = append(cs, args...)
	// when you run go test
	// go will actually run a binary of the program
	// so os.Args[0] is the actual baniary path of the current program
	/**
		$ ps -aux | grep go
		$ go test -v
	/tmp/go-build498058748/b001/goci.test -test.v=true -test.timeout=10m0s
	/tmp/go-build498058748/b001/goci.test -test.run=TestHelperProcess git push
	origin master
	*/
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	// this envs ensures the test isn't skipped
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}

	return cmd
}

func mockCmdTimeout(ctx context.Context, exe string, args ...string) *exec.Cmd {
	fmt.Println("Mock command used timeout used")
	cmd := mockCmdContext(ctx, exe, args...)
	// indicates it should run a long running process
	cmd.Env = append(cmd.Env, "GO_HELPER_TIMEOUT=1")

	return cmd
}

func TestHelperProcess(t *testing.T) {
	// this is preventing the execution if it was not called from the
	// mock command
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// simulate a log running process if the variable is set to 1
	if os.Getenv("GO_HELPER_TIMEOUT") == "1" {
		time.Sleep(15 * time.Second)
	}

	if os.Args[2] == "git" {
		fmt.Fprintln(os.Stdout, "Everything up-to-date")
		os.Exit(0)
	}
	os.Exit(1)

}

func setupGit(t *testing.T, proj string) func() {
	t.Helper()
	// check if git command exists
	gitExec, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	tempDir, err := os.MkdirTemp("", "gocitest")
	if err != nil {
		t.Fatal(err)
	}

	projPath, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	// the uri of the mock git server
	remoteURI := fmt.Sprintf("file://%s", tempDir)

	var gitCmdList = []struct {
		args []string
		dir  string
		env  []string
	}{
		{[]string{"init", "--bare"}, tempDir, nil},
		{[]string{"init"}, projPath, nil},
		{[]string{"remote", "add", "origin", remoteURI}, projPath, nil},
		{[]string{"add", "."}, projPath, nil},
		{[]string{"commit", "-m", "test"}, projPath,
			[]string{
				"GIT_COMMITER_NAME=test",
				"GIT_COMMITER=test@example.com",
				"GIT_AUTHOR_NAME=test",
				"GIT_AUTHOR_EMAIL=test@example.com",
			}},
	}

	for _, g := range gitCmdList {
		gitCmd := exec.Command(gitExec, g.args...)
		gitCmd.Dir = g.dir

		if g.env != nil {
			gitCmd.Env = append(os.Environ(), g.env...)
		}

		if err := gitCmd.Run(); err != nil {
			t.Errorf("error running git command %s, : %q", g.args, err)
		}
	}
	return func() {
		os.RemoveAll(tempDir)
		os.RemoveAll(filepath.Join(projPath, ".git"))
	}
}
