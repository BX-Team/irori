package irori_test

import (
	"strings"
	"testing"

	"github.com/bx-team/irori/internal/confs"
)

// There is deliberately no hand-written catalog for third-party cores: a key's
// documentation is the comment its own file carries. If that link breaks, the
// Configs tab shows a wall of undocumented keys and nobody notices in review.
func TestParseTakesDescriptionsFromTheFilesOwnComments(t *testing.T) {
	raw := []byte("# Paper global configuration\n" +
		"chunk-system:\n" +
		"  # Threads used to load and generate chunks.\n" +
		"  # Zero means one per core.\n" +
		"  worker-threads: 4\n" +
		"messages:\n" +
		"  kick:\n" +
		"    # Shown when the server is full\n" +
		"    server-full: 'The server is full'\n")

	doc, err := confs.Parse("config/paper-global.yml", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	workers, ok := doc.Entry("chunk-system.worker-threads")
	if !ok {
		t.Fatalf("nested key was not flattened to a dotted path, got %v", keys(doc))
	}
	if workers.Value != "4" {
		t.Errorf("worker-threads = %q, want 4", workers.Value)
	}
	if !strings.Contains(workers.Desc, "Threads used to load and generate chunks") {
		t.Errorf("description did not come from the file's comment, got %q", workers.Desc)
	}

	if _, ok := doc.Entry("messages.kick.server-full"); !ok {
		t.Errorf("a key two levels down was not flattened, got %v", keys(doc))
	}
}

// server.properties is the one file with a catalog, because it has no comments
// worth reading — the types, ranges and enums have to come from somewhere.
func TestParsePropertiesTypesKeysFromTheCatalog(t *testing.T) {
	raw := []byte("difficulty=normal\nmax-players=40\npvp=true\n")

	doc, err := confs.Parse("server.properties", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	difficulty, ok := doc.Entry("difficulty")
	if !ok {
		t.Fatalf("difficulty is missing, got %v", keys(doc))
	}
	if len(difficulty.Values) == 0 {
		t.Errorf("difficulty has no enum values from the catalog")
	}
	if difficulty.Desc == "" {
		t.Errorf("difficulty has no description from the catalog")
	}
}

func keys(d *confs.Doc) []string {
	var out []string
	for _, e := range d.Entries() {
		out = append(out, e.Key)
	}
	return out
}
