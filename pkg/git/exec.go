// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var errCmdNotInstalled = errors.New("command not installed")

type command struct {
	cmd         string
	defaultArgs []string
	timeout     time.Duration
}

func newCommand(cmd string, defaultArgs ...string) (*command, error) {
	c := &command{
		cmd: cmd,
		// Initial clone might take a while.
		timeout:     10 * time.Minute,
		defaultArgs: defaultArgs,
	}
	return c, c.verify() //nolint:gocritic
}

// Run starts the command, waits until it finished and returns stdOut or an error containing the stdError message.
func (c *command) run(args ...string) (string, error) {
	cmd := exec.Command(c.cmd, append(c.defaultArgs, args...)...) //nolint:gosec

	if v, ok := os.LookupEnv("DEBUG"); ok && v == "true" {
		fmt.Println("running: ", cmd.String())
	}

	var (
		stdOut,
		stdErr bytes.Buffer
	)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return "", errors.Wrap(err, strings.TrimSpace(stdErr.String()))
	}

	done := make(chan error)
	go func() { done <- cmd.Wait() }()

	timeout := time.After(c.timeout)
	select {
	case <-timeout:
		if err := cmd.Process.Kill(); err != nil {
			fmt.Println("failed to kill command: ", err.Error())
			return strings.TrimSpace(stdOut.String()), err
		}
		return "", fmt.Errorf("command timed out after %s: %s", time.Since(start).String(), cmd.String())
	case err := <-done:
		if stdErr.Len() > 0 {
			fmt.Println("Output:", strings.TrimSpace(stdErr.String()))
		}
		if err != nil {
			return strings.TrimSpace(stdOut.String()), errors.Wrapf(err, "command returned non-zero exit code: %s", cmd.String())
		}
	}
	return strings.TrimSpace(stdOut.String()), nil
}

// verify checks if the command is installed.
func (c *command) verify() error {
	res, err := c.run("version")
	if err != nil && strings.Contains(err.Error(), "not found") || strings.Contains(res, "not found") {
		return errCmdNotInstalled
	}

	return nil
}
