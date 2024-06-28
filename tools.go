// VSCODE will say this is an error, but it's fine
//go:build codegen
// +build codegen

package tools

import (
	// the puropse of this import is to get gomod to bring in code generator and deps
	_ "k8s.io/code-generator"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
