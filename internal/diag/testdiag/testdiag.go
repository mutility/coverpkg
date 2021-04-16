// package testdiag adapts a testing.TB to a diag.Interface or diag.Context.
package testdiag

import (
	"context"
	"testing"

	"github.com/mutility/coverpkg/internal/diag"
)

type testDiag struct {
	testing.TB
}

// Interface returns a diag.Interface that logs to t
func Interface(tb testing.TB) diag.Interface {
	return testDiag{tb}
}

// Context returns a diag.Context that logs to t and uses context.Background
func Context(t *testing.T) diag.Context {
	return WithContext(context.Background(), t)
}

// Context returns a diag.Context that logs to t and uses the specified context
func WithContext(ctx context.Context, t *testing.T) diag.Context {
	return diag.WithContext(ctx, Interface(t))
}

func (d testDiag) tb() testing.TB { return (testing.TB)(d) }

func (d testDiag) Debug(args ...interface{})   { d.tb().Helper(); d.tb().Log(args...) }
func (d testDiag) Warning(args ...interface{}) { d.tb().Helper(); d.tb().Log(args...) }
func (d testDiag) Error(args ...interface{})   { d.tb().Helper(); d.tb().Log(args...) }
