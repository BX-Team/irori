package install

import (
	"fmt"

	"github.com/bx-team/irori/internal/plugins"
)

type Issue struct {
	Target string
	Reason string
}

func (i Issue) String() string { return i.Target + ": " + i.Reason }

// Sync reconciles the lock with what is actually in the directory without
// touching the network: it fills in the checksums it can compute itself and
// reports what only a download could settle.
func Sync(t Target) ([]Issue, error) {
	var issues []Issue
	changed := t.syncCore(&issues)

	installed, _ := plugins.Scan(t.H, t.AddonDir())
	for _, item := range plugins.Reconcile(t.Cfg, t.Lock, installed) {
		switch item.State {
		case plugins.StateMissing:
			issues = append(issues, Issue{item.Name, "declared but not installed, run irori apply"})
		case plugins.StateOutdated:
			issues = append(issues, Issue{item.Name, "the declared version is not the one in the lock, run irori apply"})
		case plugins.StateUntracked:
			issues = append(issues, Issue{item.Name, "in the directory but declared nowhere, so nothing will fetch it"})
		case plugins.StateOrphan:
			issues = append(issues, Issue{item.Name, "in the lock but no longer declared, run irori apply"})
		}
	}

	if !changed {
		return issues, nil
	}
	return issues, t.Lock.Save()
}

func (t Target) syncCore(issues *[]Issue) bool {
	jar := t.Cfg.Server.Jar
	if jar == "" {
		return false
	}
	entry := t.Lock.Core
	if entry == nil {
		*issues = append(*issues, Issue{jar, "no core in the lock, run irori apply to record which build it is"})
		return false
	}
	if entry.File != jar {
		*issues = append(*issues, Issue{jar,
			fmt.Sprintf("the lock records %s as the core, run irori apply", entry.File)})
	}

	stat, err := t.H.Stat(entry.File)
	if err != nil {
		*issues = append(*issues, Issue{entry.File, "recorded in the lock but missing from the directory"})
		return false
	}
	if !entry.Direct {
		return false
	}
	// The size is the cheap half of the comparison: a jar that was swapped by
	// hand no longer matches the URL the lock would hand to Nix, and rehashing
	// every core on every render is not worth it to find that out.
	if entry.SHA256 != "" && entry.Size == stat.Size {
		return false
	}
	sum, size, err := HashFile(t.H, entry.File)
	if err != nil {
		*issues = append(*issues, Issue{entry.File, "could not be read: " + err.Error()})
		return false
	}
	if entry.SHA256 != "" && entry.SHA256 != sum {
		*issues = append(*issues, Issue{entry.File,
			"is not the jar the lock recorded, so its URL no longer describes it, run irori apply"})
		return false
	}
	entry.SHA256, entry.Size = sum, size
	return true
}
