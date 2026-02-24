package server

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Executor runs shell commands on a target system.
type Executor interface {
	Run(cmd string) (string, error)
	Close()
}

// LocalExecutor runs commands on the local machine via sh.
type LocalExecutor struct{}

func (e *LocalExecutor) Run(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return "", fmt.Errorf("local exec %q: %w", cmd, err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func (e *LocalExecutor) Close() {}
