package deploy

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type command struct {
	Name string
	Args []string
	Dir  string
}

type commandRunner interface {
	Run(context.Context, command, io.Writer, io.Writer) error
	Output(context.Context, command) (string, error)
	LookPath(string) (string, error)
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, spec command, stdout, stderr io.Writer) error {
	process := exec.CommandContext(ctx, spec.Name, spec.Args...)
	process.Dir = spec.Dir
	process.Stdout = stdout
	process.Stderr = stderr
	process.Stdin = os.Stdin
	return process.Run()
}

func (osCommandRunner) Output(ctx context.Context, spec command) (string, error) {
	process := exec.CommandContext(ctx, spec.Name, spec.Args...)
	process.Dir = spec.Dir
	output, err := process.CombinedOutput()
	return string(output), err
}

func (osCommandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
