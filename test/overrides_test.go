package irori_test

import (
	"strings"
	"testing"

	"github.com/bx-team/irori/internal/overrides"
)

// The comments in a core's config files are the only documentation irori has
// for their keys, so a writer that drops them takes the Configs tab down with
// it. Every format must come back with its comments intact.
func TestEditPreservesComments(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		raw    string
		values map[string]any
		// comments that must survive, and the line the edit must produce
		comments []string
		want     string
	}{
		{
			name: "properties",
			file: "server.properties",
			raw: "#Minecraft server properties\n" +
				"#Sat Aug 05 02:10:44 UTC 2026\n" +
				"motd=A Minecraft Server\n" +
				"server-port=25565\n",
			values:   map[string]any{"motd": "survival"},
			comments: []string{"#Minecraft server properties", "#Sat Aug 05 02:10:44 UTC 2026"},
			want:     "motd=survival",
		},
		{
			name: "yaml",
			file: "config/paper-global.yml",
			raw: "# Paper global configuration\n" +
				"chunk-system:\n" +
				"  # Threads used to load and generate chunks\n" +
				"  worker-threads: 4\n",
			values:   map[string]any{"chunk-system.worker-threads": 8},
			comments: []string{"# Paper global configuration", "# Threads used to load and generate chunks"},
			want:     "worker-threads: 8",
		},
		{
			name: "toml",
			file: "velocity.toml",
			raw: "# What port should the proxy be bound to?\n" +
				"bind = \"0.0.0.0:25577\"\n\n" +
				"[servers]\n" +
				"# A list of servers to try\n" +
				"try = [\"lobby\"]\n",
			values:   map[string]any{"bind": "0.0.0.0:25565"},
			comments: []string{"# What port should the proxy be bound to?", "# A list of servers to try"},
			want:     `bind = "0.0.0.0:25565"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, changes, err := overrides.Edit(c.file, []byte(c.raw), c.values)
			if err != nil {
				t.Fatalf("Edit: %v", err)
			}
			if len(changes) != 1 {
				t.Fatalf("got %d changes, want 1: %+v", len(changes), changes)
			}

			text := string(out)
			for _, comment := range c.comments {
				if !strings.Contains(text, comment) {
					t.Errorf("comment %q was dropped:\n%s", comment, text)
				}
			}
			if !strings.Contains(text, c.want) {
				t.Errorf("edit did not produce %q:\n%s", c.want, text)
			}
		})
	}
}

// Writing a value that is already there would rewrite the file — and its
// timestamps — on every `irori apply`.
func TestEditReportsNothingWhenTheValueMatches(t *testing.T) {
	raw := []byte("motd=survival\nserver-port=25565\n")

	out, changes, err := overrides.Edit("server.properties", raw, map[string]any{"motd": "survival"})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("got %d changes for an unchanged value, want 0: %+v", len(changes), changes)
	}
	if string(out) != string(raw) {
		t.Errorf("file was rewritten for an unchanged value:\n%s", out)
	}
}
