package main

import (
	"testing"

	"github.com/mutility/coverpkg/internal/diag"
)

func TestFullDiagInterface(t *testing.T) {
	var impl interface{} = (*GitHubAction)(nil)
	if _, ok := impl.(diag.FullInterface); !ok {
		t.Error("myimpl doesn't implement diag.FullInterface")
	}
}
