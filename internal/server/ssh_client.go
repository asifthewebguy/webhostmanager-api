package server

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHExecutor runs commands on a remote machine over SSH.
type SSHExecutor struct {
	client *ssh.Client
}

// NewSSHExecutor establishes an SSH connection and returns an executor.
func NewSSHExecutor(host string, port int, user, authType, key, password string) (*SSHExecutor, error) {
	var authMethods []ssh.AuthMethod
	switch authType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(key))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default: // "password"
		authMethods = append(authMethods, ssh.Password(password))
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec — known_hosts planned for Phase 8
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}
	return &SSHExecutor{client: client}, nil
}

func (e *SSHExecutor) Run(cmd string) (string, error) {
	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("SSH run %q: %w", cmd, err)
	}
	return string(bytes.TrimSpace(buf.Bytes())), nil
}

func (e *SSHExecutor) Close() {
	if e.client != nil {
		_ = e.client.Close()
	}
}
