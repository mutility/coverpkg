package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/urfave/cli/v2"

	"github.com/mutility/coverpkg/internal/coverage"
	"github.com/mutility/coverpkg/internal/notes"
	"github.com/mutility/diag"
)

type errInvalidGroupBy string

func (e errInvalidGroupBy) Error() string {
	return fmt.Sprintf("group-by value '%s'; must be file, package, root, or module", string(e))
}

type errInvalidComment string

func (e errInvalidComment) Error() string {
	return fmt.Sprintf("comment value '%s'; must be none, append, replace, or update", string(e))
}

type config struct {
	// Always set to true when GitHub Actions is running the workflow. You can use this variable to differentiate when tests are being run locally or by GitHub Actions.
	GithubActions bool
	// The name of the workflow.
	Workflow string
	// The name of the person or app that initiated the workflow
	Actor string
	// A unique number for each run within a repository. This number does not change if you re-run the workflow run.
	RunID string
	// The GitHub workspace directory path. The workspace directory is a copy of your repository if your workflow uses the actions/checkout action. If you don't use the actions/checkout action, the directory will be empty. For example, /home/runner/work/my-repo-name/my-repo-name.
	Workspace string
	// The owner and repository name. For example, octocat/Hello-World.
	Repository string
	// The name of the webhook event that triggered the workflow.
	EventName string
	// The path of the file with the complete webhook event payload. For example, /github/workflow/event.json.
	EventPath string
	// The commit SHA that triggered the workflow. For example, ffac537e6cbbf934b08745a378932722df287a53.
	SHA string
	// The branch or tag ref that triggered the workflow. For example, refs/heads/feature-branch-1. If neither a branch or tag is available for the event type, the variable will not exist.
	Ref string
	// Only set for pull request events. The name of the head branch.
	HeadRef string
	// Only set for pull request events. The name of the base branch.
	BaseRef string
	// Returns the URL of the GitHub server. For example: https://github.com.
	ServerURL string
	// Returns the API URL. For example: https://api.github.com.
	APIURL string
	// Returns the GraphQL API URL. For example: https://api.github.com/graphql.
	GraphQLURL string

	// File that receives environment variables to be set for future actions
	SetEnv string
	// File that receives path additions to be set for future actions
	SetPath string

	// URL for information on this run. Not set directly by github actions.
	RunURL string
	// API token for making calls to APIURL or GraphQLURL. Not set directly by github actions.
	APIToken string

	Excludes       cli.StringSlice // Package path tokens to exclude; e.g. "gen" will exclude .../gen/...
	Packages       cli.StringSlice // Packages to report on
	GroupBy        string          // file, package, root, or module
	Remote         string          // Remote that provides and/or receives coverage details
	NoPushCoverage bool            // Persist coverage details, unless true
	NoPullCoverage bool            // Retrieve coverage details, unless true
	CoverageRef    string          // Namespace for coverpkg notes
	PRComment      string          // "", update, replace, or append
	ArtifactPath   string          // Directory for artifacts; generate if unspecified.
}

func (cfg config) GitHubContext(c *cli.Context) (*GitHubAction, diag.Context) {
	gha := &GitHubAction{c.App.Writer}
	return gha, diag.WithContext(context.Background(), gha)
}

var cfg = config{
	GroupBy:     "package",
	Remote:      "origin",
	CoverageRef: "coverpkg",
}

type details struct {
	*config
	BaseSHA         string
	HeadSHA         string
	TextSummary     string
	MarkdownSummary string
	HeadPct         float64
	BasePct         float64
	DeltaPct        float64
	FoundBase       bool
	IssueNumber     int
}

func main() {
	boolVar := func(dest *bool, name, usage string, env ...string) *cli.BoolFlag {
		return &cli.BoolFlag{Name: name, EnvVars: env, Usage: usage, Destination: dest}
	}
	stringVar := func(dest *string, name, usage string, env ...string) *cli.StringFlag {
		return &cli.StringFlag{Name: name, EnvVars: env, Usage: usage, Destination: dest, DefaultText: *dest}
	}
	stringSliceVar := func(dest *cli.StringSlice, name, usage string, env ...string) *cli.StringSliceFlag {
		return &cli.StringSliceFlag{Name: name, EnvVars: env, Usage: usage, Destination: dest}
	}
	pathVar := func(dest *string, name, usage string, env ...string) *cli.PathFlag {
		return &cli.PathFlag{Name: name, EnvVars: env, Usage: usage, Destination: dest}
	}
	req := func(f cli.Flag) cli.Flag {
		switch f := f.(type) {
		case *cli.BoolFlag:
			f.Required = true
		case *cli.PathFlag:
			f.Required = true
		case *cli.StringFlag:
			f.Required = true
		default:
			panic(f)
		}
		return f
	}
	hide := func(f *cli.StringFlag) *cli.StringFlag {
		f.Hidden = true
		return f
	}
	defaultText := func(f cli.Flag, text string) cli.Flag {
		switch f := f.(type) {
		case *cli.StringFlag:
			f.DefaultText = text
		case *cli.StringSliceFlag:
			f.DefaultText = text
		default:
			panic(f)
		}
		return f
	}
	app := &cli.App{
		Name:     "coverpkg-gha",
		HelpName: "coverpkg-gha",
		Usage:    "calculate cross-package code coverage in a github-action",

		Description: `Invoke in a GitHub action as
  coverpkg-gha ${{ github.event_name }}
to automatically handle push or pull_request events.

coverpkg-gha will calculate coverage for pushed changes or pull requests. For
pull requests, the change in coverage will be shown if the base coverage can be
retrieved.`,

		// reflects https://docs.github.com/en/actions/reference/environment-variables
		Flags: []cli.Flag{
			boolVar(&cfg.GithubActions, "github-actions", "specify if running as a github action", "CI", "GITHUB_ACTIONS"),
			stringVar(&cfg.Actor, "actor", "specify who initiated the workflow", "GITHUB_ACTOR"),
			stringVar(&cfg.Workflow, "workflow", "specify the workflow name", "GITHUB_WORKFLOW"),
			stringVar(&cfg.RunID, "run-id", "specify the run-id, used to form run-url", "GITHUB_RUN_ID"),
			pathVar(&cfg.Workspace, "workspace", "specify the workspace directory", "GITHUB_WORKSPACE"),
			stringVar(&cfg.Repository, "repository", "specify the owner/repository", "GITHUB_REPOSITORY"),
			stringVar(&cfg.EventName, "event-name", "specify GitHub event name", "GITHUB_EVENT_NAME"),
			pathVar(&cfg.EventPath, "event-path", "specify GitHub webhook payload file", "GITHUB_EVENT_PATH"),
			stringVar(&cfg.SHA, "sha", "specify the triggering sha", "GITHUB_SHA"),
			stringVar(&cfg.Ref, "ref", "specify the triggering branch or tag name", "GITHUB_REF"),
			stringVar(&cfg.ServerURL, "server-url", "specify the server, used to form run-url", "GITHUB_SERVER_URL"),
			stringVar(&cfg.APIURL, "api-url", "specify the api endpoint, used for making comments", "GITHUB_API_URL"),
			hide(stringVar(&cfg.GraphQLURL, "graphql-url", "specify the graphql endpoint, could be used for making comments", "GITHUB_GRAPHQL_URL")),
			defaultText(stringVar(&cfg.RunURL, "run-url", "specify url to view this run"), "calculated"),

			pathVar(&cfg.SetEnv, "env", "specify env file"),
			pathVar(&cfg.SetPath, "path", "specify path file"),

			stringVar(&cfg.GroupBy, "group-by", "specify grouping level: file, package, root, or module", "INPUT_GROUPBY"),
			stringSliceVar(&cfg.Excludes, "exclude", "list package path names to exclude", "INPUT_EXCLUDES"),
			defaultText(stringSliceVar(&cfg.Packages, "package", "list packages to report on", "INPUT_PACKAGES"), "all root level"),

			pathVar(&cfg.ArtifactPath, "artifacts", "specify artifact output directory"),
		},

		// form run-url from server-url, repository, and run-id, unless explicitly specified.
		// validate enum-ish flags
		Before: func(c *cli.Context) error {
			switch cfg.GroupBy {
			case "file", "package", "root", "module":
			default:
				return errInvalidGroupBy(cfg.GroupBy)
			}

			if c.IsSet("run-url") || !c.IsSet("server-url") || !c.IsSet("repository") || !c.IsSet("run-id") {
				return nil
			}
			return c.Set("run-url", fmt.Sprintf("%s/%s/actions/runs/%s", c.String("server-url"), c.String("repository"), c.String("run-id")))
		},

		Commands: []*cli.Command{
			{
				Name: "schedule",
				Aliases: []string{
					"check_run",
					"check_suite",
					"create",
					"delete",
					"deployment",
					"deployment_status",
					"fork",
					"gollum",
					"issue_comment",
					"issues",
					"label",
					"milestone",
					"page_build",
					"project",
					"project_card",
					"project_column",
					"public",
					"pull_request_review",
					"pull_request_review_comment",
					"registry_package",
					"release",
					"status",
					"watch",
				},
				Usage: "Does nothing; exits without an error for unsupported GitHub Action events.",
				Action: func(c *cli.Context) error {
					gha, _ := cfg.GitHubContext(c)
					gha.Debug("Unsupported event")
					return nil
				},
			},
			{
				Name:    "push",
				Aliases: []string{"workflow_dispatch", "repository_dispatch"},
				Before:  requireEventPath,
				Action:  runPush,
				Usage:   "calculate and save code coverage for the head commit",
				Description: "Calculates, saves, and pushes code coverage information for the head commit.\n" +
					"Requires the following:\n\n" +
					"  * The desired commit has been checked out\n" +
					"  * Git is configured for commits\n" +
					"  * Git can push to origin\n\n" +
					"Provides the following outputs:\n\n" +
					"  * pushed-coverage=true, if pushed\n" +
					"  * summary=<coverage>, if calculated",
				Flags: []cli.Flag{
					boolVar(&cfg.NoPullCoverage, "coverpkg-nopull", "skip pulling coverage", "INPUT_NOPULL"),
					boolVar(&cfg.NoPushCoverage, "coverpkg-nopush", "skip pushing coverage", "INPUT_NOPUSH"),
					stringVar(&cfg.Remote, "coverpkg-remote", "specify an alternate remote name", "INPUT_REMOTE"),
					stringVar(&cfg.CoverageRef, "coverpkg-ref", "specify an alternate notes ref name", "INPUT_COVERPKGREF"),
				},
			},
			{
				Name:    "pull_request",
				Aliases: []string{"pull_request_target"},
				Before:  requireEventPath,
				Action:  runPR,
				Usage:   "calculate and display code coverage (and change) for the head commit",

				Flags: []cli.Flag{
					stringVar(&cfg.APIToken, "api-token", "specify the token used for commenting on pull requests", "INPUT_TOKEN"),
					req(stringVar(&cfg.HeadRef, "head-ref", "specify the head branch name of a pull-request", "GITHUB_HEAD_REF")),
					req(stringVar(&cfg.BaseRef, "base-ref", "specify the base branch name of a pull-request", "GITHUB_BASE_REF")),

					boolVar(&cfg.NoPullCoverage, "coverpkg-nopull", "skip pulling coverage", "INPUT_NOPULL"),
					stringVar(&cfg.Remote, "coverpkg-remote", "specify an alternate remote name", "INPUT_REMOTE"),
					stringVar(&cfg.CoverageRef, "coverpkg-ref", "specify an alternate notes ref name", "INPUT_COVERPKGREF"),
					stringVar(&cfg.PRComment, "coverpkg-comment", "specify commenting: update, replace, or append", "INPUT_COMMENT"),
				},
			},
			{
				Name:   "workflow_run",
				Before: requireEventPath,
				Action: runArtifactComment,
				Usage:  "comment on PRs from forks",

				Flags: []cli.Flag{
					stringVar(&cfg.APIToken, "api-token", "specify the token used for commenting on pull requests", "INPUT_TOKEN"),
					stringVar(&cfg.PRComment, "coverpkg-comment", "specify commenting: update, replace, or append", "INPUT_COMMENT"),
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		(&GitHubAction{os.Stdout}).Error(err)
		os.Exit(1)
	}
}

func requireEventPath(*cli.Context) error {
	if cfg.EventPath == "" {
		return errors.New(`Required flag "event-path" not set`)
	}
	return nil
}

func groupBy(ctx diag.Context, by string, filecov coverage.FileData) (interface {
	coverage.EachPather
	coverage.PathDetailer
}, error) {
	switch by {
	case "file":
		return filecov, nil
	case "package":
		return coverage.ByPackage(ctx, filecov), nil
	case "root":
		return coverage.ByRoot(ctx, filecov), nil
	case "module":
		return coverage.ByModule(ctx, filecov), nil
	default:
		return nil, errInvalidGroupBy(by)
	}
}

// runPush will generate coverage for the current
func runPush(c *cli.Context) error {
	gha, ctx := cfg.GitHubContext(c)
	filecov, err := coverage.CollectFiles(ctx, &coverage.TestOptions{
		Excludes: cfg.Excludes.Value(),
		Packages: cfg.Packages.Value(),
	})
	if err != nil {
		return err
	}

	cov, err := groupBy(ctx, cfg.GroupBy, filecov)
	if err != nil {
		return err
	}

	gha.SetOutput("summary-txt", coverage.Report(cov))
	gha.SetOutput("summary-md", coverage.ReportMD(cov))

	if cfg.NoPushCoverage {
		return nil
	}

	ref := notes.RemoteRef{
		Remote: cfg.Remote,
		Ref:    cfg.CoverageRef,
	}

	if !cfg.NoPullCoverage {
		err = notes.Fetch(ctx, ref)
		if err != nil {
			gha.Warning("fetching notes:", err)
		}
	}

	err = notes.EnsureUser(ctx)
	if err != nil {
		return err
	}
	err = notes.Store(ctx, ref, filecov)
	if err != nil {
		return err
	}

	err = notes.Push(ctx, ref)
	if err != nil {
		gha.Warning("pushing notes:", err)
	} else {
		gha.SetOutput("pushed-coverage", "true")
	}

	return nil
}

func runPR(c *cli.Context) error {
	switch cfg.PRComment {
	case "", "none", "append", "replace", "update":
	default:
		return errInvalidComment(cfg.PRComment)
	}

	gha, ctx := cfg.GitHubContext(c)
	ref := notes.RemoteRef{
		Remote: cfg.Remote,
		Ref:    cfg.CoverageRef,
	}

	if !cfg.NoPullCoverage {
		err := notes.Fetch(ctx, ref)
		if err != nil {
			gha.Warning("fetching notes:", err)
		}
	}

	detail := details{config: &cfg}

	event := gha.Event(cfg.EventPath)
	detail.BaseSHA = event.String(gha, "pull_request.base.sha")
	detail.HeadSHA = event.String(gha, "pull_request.head.sha")

	var basefilecov coverage.FileData
	err := notes.Load(ctx, ref, detail.BaseSHA, &basefilecov)
	if err != nil {
		gha.Warning("loading base coverage:", err)
	} else {
		detail.FoundBase = true
		gha.SetOutput("found-base", "true")
	}

	headfilecov, err := coverage.CollectFiles(ctx, &coverage.TestOptions{
		Excludes: cfg.Excludes.Value(),
		Packages: cfg.Packages.Value(),
	})
	if err != nil {
		return err
	}

	basecov, err := groupBy(ctx, cfg.GroupBy, basefilecov)
	if err != nil && len(basefilecov) > 0 {
		return err
	}
	headcov, err := groupBy(ctx, cfg.GroupBy, headfilecov)
	if err != nil {
		return err
	}
	diff := coverage.Diff(gha, basecov, headcov)
	detail.BasePct = coverage.Percent(basecov)
	detail.HeadPct = coverage.Percent(headcov)
	detail.DeltaPct = detail.HeadPct - detail.BasePct

	arts := cfg.ArtifactPath
	if arts == "" {
		arts, _ = os.MkdirTemp(os.TempDir(), "coverpkg")
	}

	detail.TextSummary = coverage.Report(diff)
	diag.Group(gha, "Coverage summary", func(gha diag.Interface) {
		diag.Print(gha, detail.TextSummary)
	})
	gha.SetOutput("summary-txt", detail.TextSummary)
	detail.MarkdownSummary = coverage.ReportMD(diff)
	gha.SetOutput("summary-md", detail.MarkdownSummary)
	if arts != "" {
		err = os.WriteFile(filepath.Join(arts, "summary.txt"), []byte(detail.TextSummary), 0o644)
		if err == nil {
			os.WriteFile(filepath.Join(arts, "summary.md"), []byte(detail.MarkdownSummary), 0o644)
		}
		if err == nil {
			gha.SetOutput("artifacts", arts)
		}
	}

	detail.IssueNumber = event.Int(ctx, "pull_request.number")
	id, err := doComment(ctx, event, &detail)
	if id != 0 {
		gha.SetOutput("comment-id", strconv.FormatInt(id, 10))
	}

	if isForbidden(err) {
		gha.SetOutput("comment-failed", "403")
		err = nil
	}
	return err
}

func runArtifactComment(c *cli.Context) error {
	switch cfg.PRComment {
	case "", "none", "append", "replace", "update":
	default:
		return errInvalidComment(cfg.PRComment)
	}

	gha, ctx := cfg.GitHubContext(c)
	detail := details{config: &cfg}

	gha.Group("Event "+cfg.EventPath, func(i diag.Interface) {
		evt, err := os.ReadFile(cfg.EventPath)
		if err == nil {
			gha.Printf("%s\n", evt)
		} else {
			gha.Print(err)
		}
	})

	event := gha.Event(cfg.EventPath)
	if ev := event.String(ctx, "workflow_run.event"); ev != "pull_request" {
		diag.Warning(ctx, "Unsupported workflow_run event:", ev)
		return nil
	}

	summary, err := getArtifact(ctx, event, "coverpkg", "summary.md", detail)
	if err == nil && summary != "" {
		detail.BaseSHA = event.String(gha, "workflow_run.pull_requests.0.base.sha")
		detail.HeadSHA = event.String(gha, "workflow_run.pull_requests.0.head.sha")
		detail.MarkdownSummary = summary

		gha.SetOutput("summary-md", summary)

		detail.IssueNumber = event.Int(gha, "workflow_run.pull_requests.0.number")
		id, err := doComment(ctx, event, &detail)
		if id != 0 {
			gha.SetOutput("comment-id", strconv.FormatInt(id, 10))
		}
		return err
	}
	return err
}
