package main

import (
	"encoding/json"
	"fmt"
	"os"

	rules "kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
)

func verifyArgs(args []string) error {
	numOfArgs := len(os.Args[1:])
	if numOfArgs != 1 {
		return fmt.Errorf("expected exactly 1 argument, got: %d", numOfArgs)
	}
	return nil
}

func main() {
	if err := verifyArgs(os.Args); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	targetFile := os.Args[1]

	if err := rules.SetupRules("ci"); err != nil {
		panic(err)
	}

	promRule, err := rules.BuildPrometheusRule("ci")
	if err != nil {
		panic(err)
	}
	b, err := json.Marshal(promRule.Spec)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(targetFile, b, 0600)
	if err != nil {
		panic(err)
	}

	// Make the file world-readable so the promtool container (running as a different UID)
	// can read it when mounted read-only.
	if err := os.Chmod(targetFile, 0644); err != nil {
		panic(err)
	}
}
