package exec

import (
	"context"
	"io"
	"os/exec"
)

type localExecutor struct{}

func NewLocalExecutor() Executor {
	return &localExecutor{}
}

func (e *localExecutor) Command(cmd string, args ...string) (Command, error) {
	return &localCommand{exec.Command(cmd, args...)}, nil
}

func (e *localExecutor) CommandContext(ctx context.Context, cmd string, args ...string) (Command, error) {
	return &localCommand{exec.CommandContext(ctx, cmd, args...)}, nil
}

type localCommand struct {
	*exec.Cmd
}

func (c *localCommand) SetStdin(r io.Reader) {
	c.Cmd.Stdin = r
}

func (c *localCommand) SetStdout(w io.Writer) {
	c.Cmd.Stdout = w
}

func (c *localCommand) SetStderr(w io.Writer) {
	c.Cmd.Stderr = w
}

func (c *localCommand) StdoutPipe() (io.Reader, error) {
	return c.Cmd.StdoutPipe()
}

func (c *localCommand) StderrPipe() (io.Reader, error) {
	return c.Cmd.StderrPipe()
}
