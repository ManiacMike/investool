package periphera

import "testing"

func TestRound2HandlesNegativeValues(t *testing.T) {
	if got := round2(-0.097); got != -0.1 {
		t.Fatalf("round2(-0.097)=%v, want -0.1", got)
	}
}
