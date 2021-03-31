package git

import (
	"fmt"
	"os/exec"

	"github.com/mutility/coverpkg/internal/diag"
)

func Config(ctx diag.Context, params ...string) (string, error) {
	return run(ctx, append([]string{"config"}, params...)...)
}

func Show(ctx diag.Context, params ...string) (string, error) {
	return run(ctx, append([]string{"show"}, params...)...)
}

func Checkout(ctx diag.Context, ref string) (string, error) {
	return run(ctx, "checkout", ref)
}

func Fetch(ctx diag.Context, remote string, args ...string) (string, error) {
	return run(ctx, append([]string{"fetch", remote}, args...)...)
}

func IsDirty(ctx diag.Context) bool {
	_, err := run(ctx, "diff", "--quiet", "--exit-code")
	return err != nil
}

func Push(ctx diag.Context, remote string, args ...string) (string, error) {
	return run(ctx, append([]string{"push", remote}, args...)...)
}

func RevParse(ctx diag.Context, ref string) (string, error) {
	return run(ctx, "rev-parse", ref)
}

func Notes(ctx diag.Context, args ...string) (string, error) {
	return run(ctx, append([]string{"notes"}, args...)...)
}

func run(ctx diag.Context, args ...string) (string, error) {
	if ctx != nil {
		iargs := make([]interface{}, 1+len(args))
		for i := range args {
			iargs[i+1] = args[i]
		}
		iargs[0] = "exec> git"
		diag.Debug(ctx, iargs...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			diag.Debug(ctx, "<exit", err.ExitCode(), "stderr: ", string(err.Stderr))
		}
		return string(out), fmt.Errorf("%s: %w", args, err)
	}
	return string(out), err
}
