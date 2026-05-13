package builder

import (
	"slices"
	"testing"
)

func TestGoBuildArgsAllowModuleResolution(t *testing.T) {
	args := goBuildArgs("/tmp/app")
	if !slices.Contains(args, "-mod=mod") {
		t.Fatalf("go build args must allow generated workers to resolve modules and write go.sum: %v", args)
	}
}
