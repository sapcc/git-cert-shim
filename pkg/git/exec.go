/*******************************************************************************
*
* Copyright 2019 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

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
