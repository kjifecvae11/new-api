package channel

import "testing"

func TestDedicatedNewAPIKeyIsNeverPassedThrough(t *testing.T) {
	if !shouldSkipPassthroughHeader("X-NewAPI-Key") {
		t.Fatal("dedicated gateway credential must not be forwarded upstream")
	}
}
