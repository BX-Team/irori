package irori_test

import (
	"testing"

	"github.com/bx-team/irori/internal/java"
)

// The floor table is the only thing standing between a server adopted from an
// existing directory and a start on whatever JDK happens to be first on PATH.
// A version must take the floor of the newest entry it reaches, never the one
// it is merely close to.
func TestRequiredForTakesTheHighestFloorReached(t *testing.T) {
	cases := []struct {
		mcVersion string
		want      int
	}{
		{"1.8.9", 8},
		{"1.16.5", 8},
		{"1.17", 16},
		{"1.17.1", 16},
		{"1.18", 17},
		{"1.20.4", 17},
		{"1.20.5", 21},
		{"1.21.4", 21},
		{"26", 25},
		{"26.1", 25},
		{"26w03a", 25}, // a snapshot leads the release it is numbered for
		{"", 0},        // 0 means "no requirement", never "Java 0"
		{"latest", 0},
	}

	for _, c := range cases {
		if got := java.RequiredFor(c.mcVersion); got != c.want {
			t.Errorf("RequiredFor(%q) = %d, want %d", c.mcVersion, got, c.want)
		}
	}
}

func TestMajorOfReadsBothVersionSchemes(t *testing.T) {
	cases := map[string]int{
		"1.8.0_402": 8, // pre-9 releases carry the major in the second field
		"11.0.22":   11,
		"17":        17,
		"21.0.2":    21,
		"garbage":   0,
	}

	for in, want := range cases {
		if got := java.MajorOf(in); got != want {
			t.Errorf("MajorOf(%q) = %d, want %d", in, got, want)
		}
	}
}
