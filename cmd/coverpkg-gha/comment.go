package main

import (
	"strings"
	"text/template"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"

	"github.com/mutility/coverpkg/internal/diag"
)

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
		issue:  event.Int(ctx, "pull_request.number"),
	}

	oldComment := prcomment.find(ctx)
	ctx.Debug("Existing comment ID:", oldComment.GetID())

	body := formatComment(ctx, detail)
	var err error
	var comment *github.IssueComment
	switch cfg.PRComment {
	case "replace":
		comment, err = prcomment.post(ctx, body)
		if err == nil {
			prcomment.delete(ctx, oldComment)
		}
	case "append":
		comment, err = prcomment.post(ctx, body)
	case "update":
		comment, err = prcomment.edit(ctx, oldComment, body)
	}
	return comment.GetID(), err
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
