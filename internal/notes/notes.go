package notes

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/mutility/coverpkg/internal/diag"
	"github.com/mutility/coverpkg/internal/git"
)

type RemoteRef struct {
	Remote string
	Ref    string
}

// Fetch copies notes from r to the local repo
func Fetch(ctx diag.Context, r RemoteRef) error {
	notes := `refs/notes/` + r.Ref
	out, err := git.Fetch(ctx, r.Remote, notes+":"+notes)
	diag.Debug(ctx, out)
	return err
}

// Push copies notes from the local repo to r
func Push(ctx diag.Context, r RemoteRef) error {
	notes := `refs/notes/` + r.Ref
	out, err := git.Push(ctx, r.Remote, notes+":"+notes)
	diag.Debug(ctx, out)
	return err
}

// Store saves data against the head commit, copying it or encoding as JSON.
// Note that copied data should be clear next, but this is not enforced here.
func Store(ctx diag.Context, r RemoteRef, data interface{}) error {
	if git.IsDirty(ctx) {
		return errors.New("workspace is dirty")
	}
	f, err := os.CreateTemp("", "cov*")
	if err != nil {
		return err
	}
	defer func() {
		if err = os.Remove(f.Name()); err != nil {
			diag.Error(ctx, "removing temp:", err)
		}
	}()
	switch data := data.(type) {
	case string:
		_, err = f.WriteString(data)
	case []byte:
		_, err = f.Write(data)
	default:
		e := json.NewEncoder(f)
		err = e.Encode(data)
	}
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	_, err = git.Notes(ctx, "--ref", r.Ref, "add", "-f", "-F", f.Name())
	return err
}

// Load attempts to retrieve notes from commit into data, copying or decoding as JSON.
func Load(ctx diag.Context, r RemoteRef, commit string, data interface{}) error {
	buf, err := git.Notes(ctx, "--ref", r.Ref, "show", commit)
	if err != nil {
		return err
	}

	switch data := data.(type) {
	case *string:
		*data = buf
	case *[]byte:
		*data = []byte(buf)
	default:
		d := json.NewDecoder(strings.NewReader(buf))
		return d.Decode(data)
	}
	return nil
}

// EnsureUser copies the user name and email from the head commit, if necessary.
// Calling Store in a GitHub action is likely to require this.
func EnsureUser(ctx diag.Context) error {
	if _, err := git.Config(ctx, "user.name"); err != nil {
		name, err := git.Show(ctx, "-s", "--format=%an", "HEAD")
		if err == nil {
			_, err = git.Config(ctx, "user.name", name)
		}
		if err != nil {
			return err
		}
	}
	if _, err := git.Config(ctx, "user.email"); err != nil {
		email, err := git.Show(ctx, "-s", "--format=%ae", "HEAD")
		if err == nil {
			_, err = git.Config(ctx, "user.email", email)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
