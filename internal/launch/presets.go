package launch

import "sort"

// Preset builds JVM flags for a given heap size. Several presets tune
// themselves to the heap, so this is a function rather than a fixed list.
type Preset struct {
	ID      string
	Name    string
	Summary string
	Flags   func(heapMB int) []string
}

var presets = map[string]Preset{
	"none": {
		ID:      "none",
		Name:    "No flags",
		Summary: "Only -Xms/-Xmx, everything else left to the JVM",
		Flags:   func(int) []string { return nil },
	},
	"aikar": {
		ID:      "aikar",
		Name:    "Aikar's Flags",
		Summary: "G1GC tuning, the long-standing default for Paper and its forks",
		Flags:   aikarFlags,
	},
}

// aikarFlags reproduces flags.sh; heaps of 12G and up get the large-heap tuning
// values Aikar documents separately.
func aikarFlags(heapMB int) []string {
	newSize, maxNewSize, regionSize, reserve, ihop := "30", "40", "8M", "20", "15"
	if heapMB >= 12*1024 {
		newSize, maxNewSize, regionSize, reserve, ihop = "40", "50", "16M", "15", "20"
	}
	return []string{
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=" + newSize,
		"-XX:G1MaxNewSizePercent=" + maxNewSize,
		"-XX:G1HeapRegionSize=" + regionSize,
		"-XX:G1ReservePercent=" + reserve,
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent=" + ihop,
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		"-Daikars.new.flags=true",
	}
}

func GetPreset(id string) Preset {
	if p, ok := presets[id]; ok {
		return p
	}
	return presets["none"]
}

func Presets() []Preset {
	out := make([]Preset, 0, len(presets))
	for _, p := range presets {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == "aikar" {
			return true
		}
		if out[j].ID == "aikar" {
			return false
		}
		return out[i].ID < out[j].ID
	})
	return out
}
