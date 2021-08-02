package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"

	"github.com/mutility/diag"
)

func loadMeta(ctx diag.Context, event *GitHubEvent, name, file string, detail details) error {
	tok := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: detail.APIToken})

	artifacts := wfartifacts{
		client: github.NewClient(oauth2.NewClient(ctx, tok)),
		owner:  event.String(ctx, "repository.owner.login"),
		repo:   event.String(ctx, "repository.name"),
	}

	art := artifacts.find(ctx, int64(event.Int(ctx, "workflow_run.id")), name)
	if art == nil {
		return nil
	}

	u := artifacts.download(ctx, art)
	resp, err := http.DefaultClient.Get(u.String())
	if err != nil {
		return err
	}

	artzip, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil
	}

	z, err := zip.NewReader(bytes.NewReader(artzip), int64(len(artzip)))
	if err != nil {
		return err
	}

	f, err := z.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()
	return json.NewDecoder(f).Decode(detail.coverdetail)
}

type wfartifacts struct {
	client *github.Client
	owner  string
	repo   string
}

func (a *wfartifacts) find(ctx diag.Context, runID int64, name string) *github.Artifact {
	opt := &github.ListOptions{PerPage: 20}
	for {
		arts, resp, err := a.client.Actions.ListWorkflowRunArtifacts(ctx, a.owner, a.repo, runID, opt)
		if err != nil {
			diag.Warning(ctx, "loading artifacts:", err)
			return nil
		}
		for _, art := range arts.Artifacts {
			if art.GetName() == name {
				return art
			}
		}
		if opt.Page = resp.NextPage; opt.Page == 0 {
			return nil
		}
	}
}

func (a *wfartifacts) download(ctx diag.Context, art *github.Artifact) *url.URL {
	url, _, err := a.client.Actions.DownloadArtifact(ctx, a.owner, a.repo, art.GetID(), true)
	if err != nil {
		diag.Warning(ctx, "sourcing artifact:", err)
		return nil
	}
	return url
}

func doComment(ctx diag.Context, event *GitHubEvent, detail *details) (int64, error) {
	if detail.PRComment != "replace" && detail.PRComment != "update" && detail.PRComment != "append" {
		ctx.Debug("skipping pr comment:", detail.PRComment)
		return 0, nil
	}

	tok := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: detail.APIToken})
	prcomment := issuecomments{
		client: github.NewClient(oauth2.NewClient(ctx, tok)),
		owner:  event.String(ctx, "repository.owner.login"),
		repo:   event.String(ctx, "repository.name"),
		issue:  detail.IssueNumber,
	}

	oldComment := prcomment.find(ctx)
	ctx.Debug("Existing comment ID:", oldComment.GetID())

	body := formatComment(ctx, detail)
	var err error
	var comment *github.IssueComment
	switch cfg.PRComment {
	case "replace":
		comment, err = prcomment.post(ctx, body)
		if err == nil && oldComment != nil {
			prcomment.delete(ctx, oldComment)
		}
	case "append":
		comment, err = prcomment.post(ctx, body)
	case "update":
		comment, err = prcomment.edit(ctx, oldComment, body)
	}
	return comment.GetID(), err
}

func isForbidden(err error) bool {
	var erresp *github.ErrorResponse
	return errors.As(err, &erresp)
}

type issuecomments struct {
	client *github.Client
	owner  string
	repo   string
	issue  int
}

func (gh *issuecomments) delete(ctx diag.Context, comment *github.IssueComment) {
	_, err := gh.client.Issues.DeleteComment(
		ctx, gh.owner, gh.repo, comment.GetID())
	if err != nil {
		diag.Warning(ctx, "deleting comment:", err)
	}
}

func (gh *issuecomments) post(ctx diag.Context, body string) (*github.IssueComment, error) {
	comment, _, err := gh.client.Issues.CreateComment(
		ctx, gh.owner, gh.repo, gh.issue, &github.IssueComment{Body: &body})
	if err != nil {
		diag.Error(ctx, "creating comment:", err)
	}
	return comment, err
}

func (gh *issuecomments) edit(ctx diag.Context, comment *github.IssueComment, body string) (*github.IssueComment, error) {
	comment, _, err := gh.client.Issues.EditComment(
		ctx, gh.owner, gh.repo, comment.GetID(), &github.IssueComment{Body: &body})
	if err != nil {
		diag.Error(ctx, "updating comment:", err)
	}
	return comment, err
}

func (gh *issuecomments) find(ctx diag.Context) *github.IssueComment {
	if cfg.PRComment != "update" && cfg.PRComment != "replace" {
		return nil
	}
	opt := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	}
	for {
		comments, resp, err := gh.client.Issues.ListComments(
			ctx, gh.owner, gh.repo, gh.issue, opt)
		if err != nil {
			diag.Warning(ctx, "reading comments:", err)
			return nil
		}
		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), "<!-- coverpkg-tag -->") {
				return comment
			}
		}
		if opt.Page = resp.NextPage; opt.Page == 0 {
			return nil
		}
	}
}

func formatComment(ctx diag.Context, detail *details) string {
	t := template.Must(template.New("comment").Parse(commentTemplate))
	sb := &strings.Builder{}
	err := t.Execute(sb, detail)
	if err != nil {
		ctx.Error("executing template:", err)
	}
	return sb.String()
}

const commentTemplate = `<!-- coverpkg-tag -->
Test coverage
{{- if .FoundBase }} change for **{{ .BaseRef }}** ({{ .BaseSHA }}) to
{{- else }} of
{{- end }} **{{ .HeadRef}}** ({{ .HeadSHA }}): **{{ .HeadPct | printf "%5.2f%%" }}**
{{- if .FoundBase }} ({{ .DeltaPct | printf "%+5.2f%%" }}){{ end }}

{{ .MarkdownSummary }}
`
