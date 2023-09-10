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

func TestAt(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}

	at := gha.At("file.go")
	at.Notice("file-notice")
	w.Want(t, "::notice file=file.go::file-notice\n")
	at.Noticef("%q", "a")
	w.Want(t, "::notice file=file.go::\"a\"\n")

	at.Line(10).Notice("line-notice")
	w.Want(t, "::notice file=file.go,line=10::line-notice\n")
	at.Lines(2, 4).Notice("linerange-notice")
	w.Want(t, "::notice file=file.go,line=2,endLine=4::linerange-notice\n")

	at.Line(20).Col(6).Notice("col-notice")
	w.Want(t, "::notice file=file.go,line=20,col=6::col-notice\n")
	at.Cols(6, 12).Notice("colrange-notice")
	w.Want(t, "::notice file=file.go,line=20,col=6,endColumn=12::colrange-notice\n")

	at.Line(0).Title("nopos").Notice("titled-notice")
	w.Want(t, "::notice file=file.go,title=nopos::titled-notice\n")
}

func TestDebug(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}
	gha.Debug("a", "b", "c")
	w.Want(t, "::debug::a b c\n")

	gha.Debugf("%q", "a")
	w.Want(t, "::debug::\"a\"\n")

	gha.Debug("a\nb")
	w.Want(t, "::debug::a%0Ab\n")

	gha.Debug("a%b")
	w.Want(t, "::debug::a%25b\n")
}

func TestPrint(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}
	gha.Print("a", "b", "c")
	w.Want(t, "a b c\n")

	gha.Printf("%q", "a")
	w.Want(t, "\"a\"\n")

	gha.Print("a\nb")
	w.Want(t, "a%0Ab\n")

	gha.Print("a%b")
	w.Want(t, "a%25b\n")
}

func TestError(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}
	gha.Error("a", "b", "c")
	w.Want(t, "::error::a b c\n")

	gha.Errorf("%q", "a")
	w.Want(t, "::error::\"a\"\n")

	gha.Error("a\nb")
	w.Want(t, "::error::a%0Ab\n")

	gha.Error("a%b")
	w.Want(t, "::error::a%25b\n")

	gha.ErrorAt("f.go", 23, 42, "a")
	w.Want(t, "::error file=f.go,line=23,col=42::a\n")

	gha.ErrorAt("f.go", 23, 0, "a")
	w.Want(t, "::error file=f.go,line=23::a\n")

	gha.ErrorAt("f.go", 0, 0, "a")
	w.Want(t, "::error file=f.go::a\n")

	gha.ErrorAtf("f.go", 0, 0, "%q", "a")
	w.Want(t, "::error file=f.go::\"a\"\n")
}

func TestWarning(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}
	gha.Warning("a", "b", "c")
	w.Want(t, "::warning::a b c\n")

	gha.Warningf("%q", "a")
	w.Want(t, "::warning::\"a\"\n")

	gha.Warning("a\nb")
	w.Want(t, "::warning::a%0Ab\n")

	gha.Warning("a%b")
	w.Want(t, "::warning::a%25b\n")

	gha.WarningAt("f.go", 23, 42, "a")
	w.Want(t, "::warning file=f.go,line=23,col=42::a\n")

	gha.WarningAt("f.go", 23, 0, "a")
	w.Want(t, "::warning file=f.go,line=23::a\n")

	gha.WarningAt("f.go", 0, 0, "a")
	w.Want(t, "::warning file=f.go::a\n")

	gha.WarningAtf("f.go", 0, 0, "%q", "a")
	w.Want(t, "::warning file=f.go::\"a\"\n")
}

func TestGroup(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}

	gha.Group("some%group", func(i diag.Interface) {
		i.Print("printed")
	})
	w.Want(t, "::group::some%25group\nprinted\n::endgroup::\n")
}

func TestMask(t *testing.T) {
	w := &output{}
	gha := &GitHubAction{w}

	gha.MaskValue("secret%value")
	w.Want(t, "::add-mask::secret%25value\n")
}

func TestSetEnv(t *testing.T) {
	withTempName(t, func(envs string) {
		withCfg(func() {
			cfg.SetEnv = envs
			w := &output{}
			gha := &GitHubAction{w}
			gha.SetEnv("name", "value")
			w.Want(t, "")
			wantFileContent(t, envs, "name=value\n")

			gha.SetEnv("name2", "value\n2")
			w.Want(t, "")
			wantFileContent(t, envs, "name=value\nname2=<<END_name2\nvalue\n2\nEND_name2\n")
		})
	})

	w := &output{}
	gha := &GitHubAction{w}
	gha.SetEnv("name", "value")
	w.Want(t, "::error::GITHUB_ENV not available\n")
}

func TestSetOutput(t *testing.T) {
	withTempName(t, func(outs string) {
		withCfg(func() {
			cfg.SetOutput = outs
			w := &output{}
			gha := &GitHubAction{w}

			gha.SetOutput("name", "value")
			w.Want(t, "")
			wantFileContent(t, outs, "name=value\n")

			gha.SetOutput("name2", "value2")
			w.Want(t, "")
			wantFileContent(t, outs, "name=value\nname2=value2\n")
		})
	})

	w := &output{}
	gha := &GitHubAction{w}
	gha.SetOutput("name", "value")
	w.Want(t, "::error::GITHUB_OUTPUT not available\n")
}

func TestSetPath(t *testing.T) {
	withTempName(t, func(paths string) {
		withCfg(func() {
			cfg.SetPath = paths
			w := &output{}
			gha := &GitHubAction{w}

			gha.AddPath("/some/dir")
			w.Want(t, "")
			wantFileContent(t, paths, "/some/dir\n")

			gha.AddPath("/another/dir")
			w.Want(t, "")
			wantFileContent(t, paths, "/some/dir\n/another/dir\n")
		})
	})

	w := &output{}
	gha := &GitHubAction{w}
	gha.AddPath("/some/dir")
	w.Want(t, "::error::GITHUB_PATH not available\n")
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

type output struct {
	strings.Builder
}

func (o *output) Want(t *testing.T, want string) {
	t.Helper()
	if got := o.String(); got != want {
		t.Errorf("stdout: got %q, want %q", got, want)
	}
	o.Reset()
}
