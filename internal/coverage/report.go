package coverage

import (
	"fmt"
	"io"
	"strings"
)

type PathDetailer interface {
	// Grouping returns a description of the grouping level
	Grouping() Grouping
	// Paths return a sorted list of known paths
	Paths() []string
	// Detail returns statement counts for the requested package
	Detail(pkg string) Counts
}

type BaseDetailer interface {
	// Paths return a sorted list of known packages
	Paths() []string
	// BaseDetail returns statement counts for the requested package
	BaseDetail(pkg string) Counts
}

type ChangeDetailer interface {
	PathDetailer
	BaseDetailer
}

type Counts struct {
	Total       int
	Covered     int
	IsAggregate bool
}

// Report creates a multi-line report with details of each package's coverage on
// a line. If there is more than one package, a total package '.' will be added.
func Report(c PathDetailer) string {
	sb := strings.Builder{}
	ReportTo(&sb, c)
	return sb.String()
}

// ReportTo writes Report to a specified Writer.
func ReportTo(w io.Writer, c PathDetailer) {
	maxName := 0
	pkgs := c.Paths()
	for _, name := range pkgs {
		if n := len(name); n > maxName {
			maxName = n
		}
	}

	if len(pkgs) > 1 {
		pkgs = append(pkgs, "*")
	}
	var btot, htot Counts

	d, _ := c.(ChangeDetailer)
	var lenHT, lenHC, lenBC int
	for i, pkg := range pkgs {
		var bd, hd Counts
		if i > 0 && i+1 == len(pkgs) {
			bd, hd = btot, htot
		} else {
			hd = c.Detail(pkg)
			if d != nil {
				bd = d.BaseDetail(pkg)
			}
			btot.Covered += bd.Covered
			btot.Total += bd.Total
			htot.Covered += hd.Covered
			htot.Total += hd.Total
		}

		if hd.Total > lenHT {
			lenHT = hd.Total
		}
		if hd.Covered > lenHC {
			lenHC = hd.Covered
		}
		if bd.Covered > lenBC {
			lenBC = bd.Covered
		}
	}
	lenHC, _ = fmt.Fprintf(io.Discard, "%d", lenHC)
	lenHT, _ = fmt.Fprintf(io.Discard, "%d", lenHT)
	lenBC, _ = fmt.Fprintf(io.Discard, "%d", lenBC)

	for i, pkg := range pkgs {
		var bd, hd Counts
		if i > 0 && i+1 == len(pkgs) {
			bd, hd = btot, htot
			pkg = "<all>:"
		} else {
			hd = c.Detail(pkg)
			if d != nil {
				bd = d.BaseDetail(pkg)
			}
			if hd.IsAggregate {
				pkg += "/...:"
			} else {
				pkg += ":"
			}
		}

		pctBase, pctHead := 0.0, 0.0
		if hd.Total != 0 {
			pctHead = float64(100*hd.Covered) / float64(hd.Total)
		}
		if bd.Total != 0 {
			pctBase = float64(100*bd.Covered) / float64(bd.Total)
		}

		// report lines of: `{pkg}: {current}  {delta}  ({old})`
		// where current/old are `percent  n of m` and include delta only
		// for ChangeDetailers, include old only if nonzero base

		if pctBase > 0 {
			fmt.Fprintf(w, "%-*s %6.2f%%  %*d of %*d %+7.2f%%  (was %6.2f%%  %*d of %d)\n",
				maxName+5, pkg,
				pctHead, lenHC, hd.Covered, lenHT, hd.Total,
				pctHead-pctBase,
				pctBase, lenBC, bd.Covered, bd.Total,
			)
		} else if d != nil {
			fmt.Fprintf(w, "%-*s %6.2f%%  %*d of %*d %+7.2f%%\n",
				maxName+5, pkg,
				pctHead, lenHC, hd.Covered, lenHT, hd.Total,
				pctHead-pctBase,
			)
		} else {
			fmt.Fprintf(w, "%-*s %6.2f%%  %*d of %d\n",
				maxName+5, pkg,
				pctHead, lenHC, hd.Covered, hd.Total,
			)
		}
	}
}

// ReportMD creates a multi-line report with details of each package's coverage on
// a line. If there is more than one package, a total package '.' will be added.
func ReportMD(c PathDetailer) string {
	sb := strings.Builder{}
	ReportMDTo(&sb, c)
	return sb.String()
}

// ReportTo writes Report to a specified Writer.
func ReportMDTo(w io.Writer, c PathDetailer) {
	pkgs := c.Paths()
	if len(pkgs) > 1 {
		pkgs = append(pkgs, "*")
	}
	var btot, htot Counts

	d, _ := c.(ChangeDetailer)
	for i, pkg := range pkgs {
		var bd, hd Counts
		if i == 0 || i+1 != len(pkgs) {
			hd = c.Detail(pkg)
			if d != nil {
				bd = d.BaseDetail(pkg)
			}
			btot.Covered += bd.Covered
			btot.Total += bd.Total
			htot.Covered += hd.Covered
			htot.Total += hd.Total
		}
	}
	grouping := "| " + c.Grouping().String()
	if btot.Total > 0 {
		fmt.Fprintln(w, grouping+" | Coverage | Statements | Change | (Covered) | (Statements) |")
		fmt.Fprintln(w, "|:--|--:|--:|--:|--:|--:|")
	} else {
		fmt.Fprintln(w, grouping+" | Coverage | Statements |")
		fmt.Fprintln(w, "|:--|--:|--:|")
	}

	for i, pkg := range pkgs {
		bd, hd := btot, htot
		if i == 0 || i+1 != len(pkgs) {
			hd = c.Detail(pkg)
			if d != nil {
				bd = d.BaseDetail(pkg)
			}
			if hd.IsAggregate {
				pkg += "/..."
			}
		} else {
			pkg = "**Total**"
		}
		if bd.Total > 0 {
			hpct := 0.0
			if hd.Total > 0 {
				hpct = float64(100*hd.Covered) / float64(hd.Total)
			}
			bpct := float64(100*bd.Covered) / float64(bd.Total)
			fmt.Fprintf(w, "%s|%.2f%%|%d of %d|%+.2f%%|(%.2f%%)|(%d of %d)\n", pkg,
				hpct, hd.Covered, hd.Total,
				hpct-bpct,
				bpct, bd.Covered, bd.Total,
			)
		} else {
			fmt.Fprintf(w, "%s|%.2f%%|%d of %d\n", pkg,
				float64(100*hd.Covered)/float64(hd.Total), hd.Covered, hd.Total,
			)
		}
	}
}
