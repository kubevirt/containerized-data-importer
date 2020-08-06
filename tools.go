// VSCODE will say this is an error, but it's fine
// +build ignore

package tools

import (
	// the puropse of this import is to get gomod to bring in code generator and deps
	_ "k8s.io/code-generator"
)
