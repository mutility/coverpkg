package coverage

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/mutility/diag/testdiag"
)

func TestLoadAgg(t *testing.T) {
	const mod = "github.com/mutility/coverpkg"
	const prof = "testdata/cover.prof"

	ctx := testdiag.Context(t)

	st, err := LoadProfile(ctx, prof, DefaultTestOptions)
	if err != nil {
		t.Error("load", err)
	}
	if len(st) != 216 {
		t.Errorf("statements: got %v, want %v", len(st), 216)
	}
	for loc := range st {
		if loc.filepos == "" {
			t.Error("loc empty")
		}
		if loc.count == 0 {
			t.Error("count 0")
		}
	}

	pkg := ByPackage(ctx, st)
	wantpkg := PackageData{
		"github.com/mutility/coverpkg/internal/coverage": StmtCount{178, 77},
		"github.com/mutility/coverpkg/internal/ghacover": StmtCount{100, 0},
		"github.com/mutility/coverpkg/internal/gitcover": StmtCount{56, 0},
		"github.com/mutility/coverpkg":                   StmtCount{25, 0},
	}

	if diff := cmp.Diff(wantpkg, pkg); diff != "" {
		t.Errorf("bypkg (-want +got):\n%s", diff)
	}

	root := ByRoot(ctx, st)
	wantroot := PackageData{
		"github.com/mutility/coverpkg/internal": StmtCount{334, 77},
		"github.com/mutility/coverpkg":          StmtCount{25, 0},
	}

	if diff := cmp.Diff(wantroot, root); diff != "" {
		t.Errorf("byroot (-want +got):\n%s", diff)
	}
}
