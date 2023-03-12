package framework

import (
	"fmt"
	"os"
	"os/exec"
)

// RunKubectlCommand runs a kubectl Cmd and returns output and err
func (f *Framework) RunKubectlCommand(args ...string) (string, error) {
	cmd := f.CreateKubectlCommand(args...)
	outBytes, err := cmd.CombinedOutput()

	return string(outBytes), err
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
