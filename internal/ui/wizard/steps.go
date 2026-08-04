package wizard

import (
	"context"
	"time"

	"github.com/bx-team/irori/internal/importer"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

type typesMsg struct {
	types []mcjars.TypeInfo
	err   error
}

type versionsMsg struct {
	versions []mcjars.Version
	err      error
}

type buildsMsg struct {
	builds []mcjars.Build
	err    error
}

type scanMsg struct {
	candidates []importer.Candidate
	script     *importer.StartScript
	err        error
}

type installProgressMsg mcjars.Progress

type installDoneMsg struct{ err error }

const apiTimeout = 30 * time.Second

func loadTypes(c *mcjars.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		types, err := c.Types(ctx)
		return typesMsg{types: types, err: err}
	}
}

func loadVersions(c *mcjars.Client, t models.ServerType) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		versions, _, err := c.Versions(ctx, t, 1, 200, "")
		return versionsMsg{versions: versions, err: err}
	}
}

func loadBuilds(c *mcjars.Client, t models.ServerType, version string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		builds, _, err := c.Builds(ctx, t, version, 1, 50)
		return buildsMsg{builds: builds, err: err}
	}
}

func scanExisting(c *mcjars.Client, dir string) tea.Cmd {
	return func() tea.Msg {
		cands, err := importer.ScanJars(dir)
		if err != nil {
			return scanMsg{err: err}
		}

		var script *importer.StartScript
		for _, p := range importer.FindStartScripts(dir) {
			if s, ok := importer.ParseStartScript(p); ok {
				script = s
				break
			}
		}

		if len(cands) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
			cands, _ = importer.Identify(ctx, c, dir, cands)
			cancel()
		}
		return scanMsg{candidates: cands, script: script}
	}
}

func (m *Model) startInstall(build mcjars.Build) tea.Cmd {
	progress := make(chan mcjars.Progress, 64)
	m.progressCh = progress

	return tea.Batch(
		waitProgress(progress),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			err := m.client.Install(ctx, build, m.dir, func(p mcjars.Progress) {
				select {
				case progress <- p:
				default:
				}
			})
			close(progress)
			return installDoneMsg{err: err}
		},
	)
}

func waitProgress(ch chan mcjars.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return installProgressMsg(p)
	}
}
