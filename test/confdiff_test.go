package irori_test

import (
	"testing"

	"github.com/bx-team/irori/internal/confdiff"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
)

// What this pass declares is re-applied on every start, on the NixOS host too.
// A version marker pinned that way freezes the core at a config schema it has
// already moved past, and a secret ends up in a world readable store path — so
// neither may ever reach .irori.json, however plainly it "differs".
func TestCompareSkipsVersionMarkersAndSecrets(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "server.properties", "difficulty=hard\nmotd=Home\nrcon.password=hunter2\n")
	write(t, dir, "config/paper-global.yml", "_version: 31\nchunk-system:\n  gen-parallelism: true\n")

	shipped := []mcjars.Config{
		{Location: "./server.properties", Format: "PROPERTIES",
			Value: "difficulty=easy\nmotd=Home\nrcon.password=\n"},
		{Location: "./config/paper-global.yml", Format: "YAML",
			Value: "_version: 29\nchunk-system:\n  gen-parallelism: default\n"},
	}

	res := confdiff.Compare(host.NewLocal(dir), shipped)

	got := map[string]confdiff.Change{}
	for _, c := range res.Changes {
		got[c.Key] = c
	}
	if _, ok := got["_version"]; ok {
		t.Error("the core's own version marker was declared")
	}
	if _, ok := got["rcon.password"]; ok {
		t.Error("a secret was declared")
	}
	if len(res.Skipped) != 2 {
		t.Errorf("got %d skipped keys, want 2: %+v", len(res.Skipped), res.Skipped)
	}

	if c, ok := got["difficulty"]; !ok || c.Value != "hard" || c.Default != "easy" {
		t.Errorf("difficulty was not reported as easy → hard, got %+v", c)
	}
	// The value goes into .irori.json as JSON, and a boolean written as the
	// string "true" comes back quoted in a yml the core then fails to read.
	if c, ok := got["chunk-system.gen-parallelism"]; !ok || c.Typed != true {
		t.Errorf("gen-parallelism came back as %#v, want the boolean true", c.Typed)
	}
}

// A file the core ships but nobody has generated yet must not be reported as
// "everything differs" — there is nothing to compare it against.
func TestCompareIgnoresFilesThatAreNotThere(t *testing.T) {
	dir := t.TempDir()
	shipped := []mcjars.Config{{Location: "./bukkit.yml", Format: "YAML", Value: "settings:\n  allow-end: true\n"}}

	res := confdiff.Compare(host.NewLocal(dir), shipped)
	if len(res.Compared) != 0 || !res.Empty() {
		t.Errorf("a missing file produced %+v", res)
	}
}

func TestDeclareStoresTypedValues(t *testing.T) {
	cfg := config.Default(t.TempDir())
	confdiff.Declare(cfg, []confdiff.Change{
		{File: "server.properties", Key: "view-distance", Typed: 10},
	})
	if got := cfg.Configs["server.properties"]["view-distance"]; got != 10 {
		t.Errorf("view-distance stored as %#v, want the number 10", got)
	}
}
