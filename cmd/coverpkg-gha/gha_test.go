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

func TestDebug(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.Debug("a", "b", "c")
	wantOutput(t, "stdout", w.String(), "::debug::a b c\n")

	w.Reset()
	gha.Debugf("%q", "a")
	wantOutput(t, "stdout", w.String(), "::debug::\"a\"\n")

	w.Reset()
	gha.Debug("a\nb")
	wantOutput(t, "stdout", w.String(), "::debug::a%0Ab\n")

	w.Reset()
	gha.Debug("a%b")
	wantOutput(t, "stdout", w.String(), "::debug::a%25b\n")
}

func TestPrint(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.Print("a", "b", "c")
	wantOutput(t, "stdout", w.String(), "a b c\n")

	w.Reset()
	gha.Printf("%q", "a")
	wantOutput(t, "stdout", w.String(), "\"a\"\n")

	w.Reset()
	gha.Print("a\nb")
	wantOutput(t, "stdout", w.String(), "a%0Ab\n")

	w.Reset()
	gha.Print("a%b")
	wantOutput(t, "stdout", w.String(), "a%25b\n")
}

func TestError(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.Error("a", "b", "c")
	wantOutput(t, "stdout", w.String(), "::error::a b c\n")

	w.Reset()
	gha.Errorf("%q", "a")
	wantOutput(t, "stdout", w.String(), "::error::\"a\"\n")

	w.Reset()
	gha.Error("a\nb")
	wantOutput(t, "stdout", w.String(), "::error::a%0Ab\n")

	w.Reset()
	gha.Error("a%b")
	wantOutput(t, "stdout", w.String(), "::error::a%25b\n")

	w.Reset()
	gha.ErrorAt("f.go", 23, 42, "a")
	wantOutput(t, "stdout", w.String(), "::error file=f.go,line=23,col=42::a\n")

	w.Reset()
	gha.ErrorAt("f.go", 23, 0, "a")
	wantOutput(t, "stdout", w.String(), "::error file=f.go,line=23::a\n")

	w.Reset()
	gha.ErrorAt("f.go", 0, 0, "a")
	wantOutput(t, "stdout", w.String(), "::error file=f.go::a\n")

	w.Reset()
	gha.ErrorAtf("f.go", 0, 0, "%q", "a")
	wantOutput(t, "stdout", w.String(), "::error file=f.go::\"a\"\n")
}

func TestWarning(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.Warning("a", "b", "c")
	wantOutput(t, "stdout", w.String(), "::warning::a b c\n")

	w.Reset()
	gha.Warningf("%q", "a")
	wantOutput(t, "stdout", w.String(), "::warning::\"a\"\n")

	w.Reset()
	gha.Warning("a\nb")
	wantOutput(t, "stdout", w.String(), "::warning::a%0Ab\n")

	w.Reset()
	gha.Warning("a%b")
	wantOutput(t, "stdout", w.String(), "::warning::a%25b\n")

	w.Reset()
	gha.WarningAt("f.go", 23, 42, "a")
	wantOutput(t, "stdout", w.String(), "::warning file=f.go,line=23,col=42::a\n")

	w.Reset()
	gha.WarningAt("f.go", 23, 0, "a")
	wantOutput(t, "stdout", w.String(), "::warning file=f.go,line=23::a\n")

	w.Reset()
	gha.WarningAt("f.go", 0, 0, "a")
	wantOutput(t, "stdout", w.String(), "::warning file=f.go::a\n")

	w.Reset()
	gha.WarningAtf("f.go", 0, 0, "%q", "a")
	wantOutput(t, "stdout", w.String(), "::warning file=f.go::\"a\"\n")
}

func TestGroup(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}

	gha.Group("some%group", func(i diag.Interface) {
		i.Print("printed")
	})
	wantOutput(t, "stdout", w.String(), "::group::some%25group\nprinted\n::endgroup::\n")
}

func TestMask(t *testing.T) {
	w := &strings.Builder{}
	gha := &GitHubAction{w}

	gha.MaskValue("secret%value")
	wantOutput(t, "stdout", w.String(), "::add-mask::secret%25value\n")
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

func TestSetPath(t *testing.T) {
	withTempName(t, func(paths string) {
		withCfg(func() {
			cfg.SetPath = paths
			w := &strings.Builder{}
			gha := &GitHubAction{w}

			gha.AddPath("/some/dir")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, paths, "/some/dir\n")

			gha.AddPath("/another/dir")
			wantOutput(t, "stdout", w.String(), "")
			wantFileContent(t, paths, "/some/dir\n/another/dir\n")
		})
	})

	w := &strings.Builder{}
	gha := &GitHubAction{w}
	gha.AddPath("/some/dir")
	wantOutput(t, "stdout", w.String(), "::error::GITHUB_PATH not available\n")
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
