package main

import (
	"os"
	"strings"
	"testing"

	"github.com/mutility/diag"
)

func TestFullDiagInterface(t *testing.T) {
	var impl interface{} = (*GitHubAction)(nil)
	if _, ok := impl.(diag.FullInterface); !ok {
		t.Error("myimpl doesn't implement diag.FullInterface")
	}
}

func TestSetEnv(t *testing.T) {
	withTempName(t, func(envs string) {
		withCfg(func() {
			cfg.SetEnv = envs
			w := &strings.Builder{}
			gha := &GitHubAction{w}
			gha.SetEnv("name", "value")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, envs, "name=value\n")

			gha.SetEnv("name2", "value\n2")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, envs, "name=value\nname2=<<END_name2\nvalue\n2\nEND_name2\n")
		})
	})

	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.SetEnv("name", "value")
	wantOutput(t, "stdout", w.String(), "::error::GITHUB_ENV not available\n")
}

func TestSetOutput(t *testing.T) {
	withTempName(t, func(outs string) {
		withCfg(func() {
			cfg.SetOutput = outs
			w := &strings.Builder{}
			gha := &GitHubAction{w}

			gha.SetOutput("name", "value")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, outs, "name=value\n")

			gha.SetOutput("name2", "value2")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, outs, "name=value\nname2=value2\n")
		})
	})

	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.SetOutput("name", "value")
	wantOutput(t, "stdout", w.String(), "::error::GITHUB_OUTPUT not available\n")
}

func withCfg(fn func()) {
	defer func(old config) { cfg = old }(cfg)
	fn()
}

func withTempName(t *testing.T, fn func(name string)) {
	t.Helper()
	outs, err := os.CreateTemp("", "gho")
	if err != nil {
		t.Fatal(err)
	}
	outs.Close()
	t.Cleanup(func() { os.Remove(outs.Name()) })
	fn(outs.Name())
}

func wantFileContent(t *testing.T, name, content string) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil {
		t.Error("reading content:", err)
	}
	if string(got) != content {
		t.Errorf("content: got %q want %q", got, content)
	}
}

func wantOutput(t *testing.T, label, got, want string) {
	if got != want {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}
