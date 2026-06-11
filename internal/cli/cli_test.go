package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bibimoni/orphion/internal/cli"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := cli.New(nil)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "orphion") {
		t.Errorf("version output should mention orphion, got: %s", out)
	}
}
