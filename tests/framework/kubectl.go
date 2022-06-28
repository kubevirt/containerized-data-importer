package framework

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

//RunKubectlCommand runs a kubectl Cmd and returns output and err
func (f *Framework) RunKubectlCommand(args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := f.CreateKubectlCommand(args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
		// err will not always be nil calling kubectl, this is expected on no results for instance.
		// still return the value and let the caller decide what to do.
		return string(stdOutBytes), err
	}
	return string(stdOutBytes), nil
}

// CreateKubectlCommand returns the Cmd to execute kubectl
func (f *Framework) CreateKubectlCommand(args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}
