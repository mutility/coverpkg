package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/mutility/coverpkg/internal/coverage"
	"github.com/mutility/coverpkg/internal/diag"
	"github.com/mutility/coverpkg/internal/notes"
)

type config struct {
	// BaseRef lists a base committish for comparisons.
	BaseRef string
	// BaseProfile lists a base coverprofile for comparisons.
	BaseProfile string

	// StoreCoverage controls if the calculation will be persisted in git.
	StoreCoverage bool

	// List of package path tokens to exclude; e.g. "gen" will exclude .../gen/...
	Excludes cli.StringSlice

	// List of packages to report on
	Packages cli.StringSlice

	Debug       bool
	GroupBy     string // aggregation level, "file", "package", "root" or "module"
	Format      string // format of output, "ascii" or "markdown"
	CoverageRef string // Namespace for coverpkg notes
}

var cfg = config{
	GroupBy:     "package",
	Format:      "ascii",
	CoverageRef: "coverpkg",
}

func (cfg config) Context(c *cli.Context) diag.Context {
	var log diag.Interface = diag.New(c.App.Writer)
	if cfg.Debug {
		log = diag.NewDebug(c.App.Writer)
	}
	return diag.WithContext(context.Background(), log)
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
		Name:     "coverpkg",
		HelpName: "coverpkg",
		Usage:    "calculate cross-package code coverage",

		Description: ``,

		// reflects https://docs.github.com/en/actions/reference/environment-variables
		Flags: []cli.Flag{
			stringSliceVar(&cfg.Excludes, "exclude", "list package path names to exclude", "INPUT_EXCLUDES"),
			defaultText(stringSliceVar(&cfg.Packages, "package", "list packages to report on", "INPUT_EXCLUDES"), "all root level"),
			boolVar(&cfg.Debug, "debug", "enable debug messages", "COVERPKG_DEBUG"),
		},

		Commands: []*cli.Command{
			{
				Name:   "calc",
				Action: runCalc,
				Usage:  "calculate and display code coverage",
				Flags: []cli.Flag{
					stringVar(&cfg.GroupBy, "g", "specify grouping: file, package, root, or module", "COVERPKG_BY"),
					stringVar(&cfg.Format, "f", "specify format: <ascii> art or <markdown>", "COVERPKG_FMT"),
					boolVar(&cfg.StoreCoverage, "store", "store coverage info to git, useful to enable diff"),
					stringVar(&cfg.CoverageRef, "coverpkg-ref", "specify an alternate notes ref name", "INPUT_COVERPKGREF"),
				},
			},
			{
				Name:   "diff",
				Action: runDiff,
				Usage:  "calculate and display code coverage and change",

				Flags: []cli.Flag{
					stringVar(&cfg.GroupBy, "g", "specify grouping: file, package, root, or module", "COVERPKG_BY"),
					stringVar(&cfg.Format, "f", "specify format: <ascii> art or <markdown>", "COVERPKG_FMT"),

					stringVar(&cfg.BaseRef, "base-ref", "specify the base branch or commit hash"),
					pathVar(&cfg.BaseProfile, "base-coverprofile", "specify the base coverprofile"),

					stringVar(&cfg.CoverageRef, "coverpkg-ref", "specify an alternate notes ref name", "INPUT_COVERPKGREF"),
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runCalc will generate coverage for the current
func runCalc(c *cli.Context) error {
	ctx := cfg.Context(c)

	filecov, err := coverage.CollectFiles(ctx, &coverage.TestOptions{
		Excludes: cfg.Excludes.Value(),
		Packages: cfg.Packages.Value(),
	})
	if err != nil {
		return err
	}

	var cov coverage.PathDetailer
	switch cfg.GroupBy {
	case "file":
		cov = filecov
	case "package":
		cov = coverage.ByPackage(ctx, filecov)
	case "root":
		cov = coverage.ByRoot(ctx, filecov)
	case "module":
		cov = coverage.ByModule(ctx, filecov)
	}

	switch cfg.Format {
	case "markdown":
		fmt.Print(coverage.ReportMD(cov))
	default:
		fmt.Print(coverage.Report(cov))
	}

	if cfg.StoreCoverage {
		ref := notes.RemoteRef{Ref: cfg.CoverageRef}
		return notes.Store(ctx, ref, filecov)
	}

	return nil
}

func runDiff(c *cli.Context) error {
	ctx := cfg.Context(c)
	ref := notes.RemoteRef{Ref: cfg.CoverageRef}
	options := &coverage.TestOptions{
		Excludes: cfg.Excludes.Value(),
		Packages: cfg.Packages.Value(),
	}

	var basefilecov coverage.FileData
	if cfg.BaseRef != "" {
		err := notes.Load(ctx, ref, cfg.BaseRef, &basefilecov)
		if err != nil {
			return fmt.Errorf("loading base ref: %w", err)
		}
	} else if cfg.BaseProfile != "" {
		stmts, err := coverage.LoadProfile(ctx, cfg.BaseProfile, options)
		if err != nil {
			return fmt.Errorf("loading base coverprofile: %w", err)
		}
		basefilecov = coverage.ByFiles(ctx, stmts)
	}

	headfilecov, err := coverage.CollectFiles(ctx, options)
	if err != nil {
		return err
	}

	basepkgcov := coverage.ByPackage(ctx, basefilecov)
	headpkgcov := coverage.ByPackage(ctx, headfilecov)
	pkgdelta := coverage.Diff(ctx, headpkgcov, basepkgcov)

	switch cfg.Format {
	case "markdown":
		fmt.Print(coverage.ReportMD(pkgdelta))
	default:
		fmt.Print(coverage.Report(pkgdelta))
	}

	return nil
}
