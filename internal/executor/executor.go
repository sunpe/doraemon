package executor

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type Result struct {
	Stdout     []byte
	Stderr     []byte
	ExitCode   int
	DurationMS int64
}

func Run(ctx context.Context, timeout time.Duration, command string, args []string, maxStdout, maxStderr int64) Result {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout, stderr limitBuffer
	stdout.limit = maxStdout
	stderr.limit = maxStderr
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode, DurationMS: time.Since(start).Milliseconds()}
}

type limitBuffer struct {
	bytes.Buffer
	limit int64
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return b.Buffer.Write(p)
	}
	remaining := b.limit - int64(b.Buffer.Len())
	if remaining <= 0 {
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
