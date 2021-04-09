package coverage_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/mutility/coverpkg/internal/coverage"
)

type stmts struct {
	Pkg string
	Cov int
	Tot int
}

type pkgs []stmts

func (p pkgs) Paths() []string {
	pkgs := make([]string, len(p))
	for i := range p {
		pkgs[i] = p[i].Pkg
	}
	return pkgs
}

func (p pkgs) Detail(pkg string) coverage.Counts {
	for _, p := range p {
		if p.Pkg == pkg {
			return coverage.Counts{Total: p.Tot, Covered: p.Cov, IsAggregate: pkg != "."}
		}
	}
	return coverage.Counts{}
}

type deltas struct {
	Pkg              string
	BaseTot, HeadTot int
	BaseCov, HeadCov int
}

type dpkgs []deltas

func (p dpkgs) Paths() []string {
	pkgs := make([]string, len(p))
	for i := range p {
		pkgs[i] = p[i].Pkg
	}
	return pkgs
}

func (p dpkgs) Detail(pkg string) coverage.Counts {
	for _, p := range p {
		if p.Pkg == pkg {
			return coverage.Counts{Total: p.HeadTot, Covered: p.HeadCov, IsAggregate: pkg != "."}
		}
	}
	return coverage.Counts{}
}

func (p dpkgs) BaseDetail(pkg string) coverage.Counts {
	for _, p := range p {
		if p.Pkg == pkg {
			return coverage.Counts{Total: p.BaseTot, Covered: p.BaseCov}
		}
	}
	return coverage.Counts{}
}

func scov(pkg string, cov, total int) stmts {
	sc := stmts{}
	sc.Pkg = pkg
	sc.Cov = cov
	sc.Tot = total
	return sc
}

func sdcov(pkg string, bcov, btot, hcov, htot int) deltas {
	return deltas{pkg, btot, htot, bcov, hcov}
}

func TestReport(t *testing.T) {
	mdhead := "| Package | Coverage | Statements |\n|:--|--:|--:|\n"
	mddiff := "| Package | Coverage | Statements | Change | (Covered) | (Statements) |\n|:--|--:|--:|--:|--:|--:|\n"
	for _, tt := range []struct {
		name   string
		want   string
		wantmd string
		cov    coverage.PathDetailer
	}{
		{
			"one", "pkg/...:  70.00%  7 of 10\n",
			mdhead + "pkg/...|70.00%|7 of 10\n",
			pkgs{scov("pkg", 7, 10)},
		},
		{
			"two", "" +
				"a/...:  70.00%   7 of 10\n" +
				"b/...:  23.08%   3 of 13\n" +
				"<all>:  43.48%  10 of 23\n",
			mdhead +
				"a/...|70.00%|7 of 10\n" +
				"b/...|23.08%|3 of 13\n" +
				"**Total**|43.48%|10 of 23\n",
			pkgs{scov("a", 7, 10), scov("b", 3, 13)},
		},
		{
			"dot", "" +
				"sub/...:  80.00%  8 of 10\n" +
				".:         0.00%  0 of 2\n" +
				"<all>:    66.67%  8 of 12\n",
			mdhead +
				"sub/...|80.00%|8 of 10\n" +
				".|0.00%|0 of 2\n" +
				"**Total**|66.67%|8 of 12\n",
			pkgs{scov("sub", 8, 10), scov(".", 0, 2)},
		},
		{
			"delta", "pkg/...:  63.64%  7 of 11  -14.14%  (was  77.78%  7 of 9)\n",
			mddiff + "pkg/...|63.64%|7 of 11|-14.14%|(77.78%)|(7 of 9)\n",
			dpkgs{sdcov("pkg", 7, 9, 7, 11)},
		},
		{
			"nobase", "pkg/...:  63.64%  7 of 11  +63.64%\n",
			mdhead + "pkg/...|63.64%|7 of 11\n",
			dpkgs{sdcov("pkg", 0, 0, 7, 11)},
		},
		{
			"complex", "" +
				"new/...:        1.14%    1 of  88   +1.14%\n" +
				"match/...:     88.89%   88 of  99   +0.00%  (was  88.89%   88 of 99)\n" +
				"improve/...:   80.00%   80 of 100  +20.00%  (was  60.00%   60 of 100)\n" +
				"decrease/...:  20.00%   20 of 100  -20.00%  (was  40.00%   20 of 50)\n" +
				"unlikely/...: 100.00%    1 of   1   +0.00%  (was 100.00%    1 of 1)\n" +
				"<all>:         48.97%  190 of 388  -18.63%  (was  67.60%  169 of 250)\n",
			mddiff +
				"new/...|1.14%|1 of 88\n" +
				"match/...|88.89%|88 of 99|+0.00%|(88.89%)|(88 of 99)\n" +
				"improve/...|80.00%|80 of 100|+20.00%|(60.00%)|(60 of 100)\n" +
				"decrease/...|20.00%|20 of 100|-20.00%|(40.00%)|(20 of 50)\n" +
				"unlikely/...|100.00%|1 of 1|+0.00%|(100.00%)|(1 of 1)\n" +
				"**Total**|48.97%|190 of 388|-18.63%|(67.60%)|(169 of 250)\n",
			dpkgs{
				sdcov("new", 0, 0, 1, 88),
				sdcov("match", 88, 99, 88, 99),
				sdcov("improve", 60, 100, 80, 100),
				sdcov("decrease", 20, 50, 20, 100),
				sdcov("unlikely", 1, 1, 1, 1),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := coverage.Report(tt.cov)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("report (-want +got):\n%s", diff)
			}
			gotmd := coverage.ReportMD(tt.cov)
			if diff := cmp.Diff(tt.wantmd, gotmd); diff != "" {
				t.Errorf("reportmd (-want +got):\n%s", diff)
			}
		})
	}
}
