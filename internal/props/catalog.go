package props

type Kind string

const (
	KindBool   Kind = "bool"
	KindInt    Kind = "int"
	KindEnum   Kind = "enum"
	KindString Kind = "string"
	// KindFloat only ever comes from a config file that already holds a
	// fractional number; server.properties has no such key.
	KindFloat Kind = "float"
)

type Spec struct {
	Key     string
	Kind    Kind
	Default string
	Section string
	Desc    string
	Values  []string
	Min     int
	Max     int
	Secret  bool
}

const (
	SecServer   = "Server"
	SecGameplay = "Gameplay"
	SecWorld    = "World"
	SecPlayers  = "Players"
	SecPerf     = "Performance"
	SecPack     = "Resource pack"
	SecRemote   = "Query & RCON"
	SecAdvanced = "Advanced"
	SecOther    = "Other"
)

// Sections is the display order; anything not listed here sorts last.
var Sections = []string{
	SecServer, SecGameplay, SecWorld, SecPlayers,
	SecPerf, SecPack, SecRemote, SecAdvanced, SecOther,
}

var Catalog = []Spec{
	{Key: "motd", Kind: KindString, Default: "A Minecraft Server", Section: SecServer,
		Desc: "Message shown in the multiplayer server list. Supports § colour codes."},
	{Key: "server-port", Kind: KindInt, Default: "25565", Section: SecServer, Min: 1, Max: 65535,
		Desc: "TCP port the server listens on."},
	{Key: "server-ip", Kind: KindString, Default: "", Section: SecServer,
		Desc: "Address to bind to. Leave empty to listen on every interface."},
	{Key: "max-players", Kind: KindInt, Default: "20", Section: SecServer, Min: 0, Max: 1000000,
		Desc: "How many players may be connected at once."},
	{Key: "online-mode", Kind: KindBool, Default: "true", Section: SecServer,
		Desc: "Verify players against Mojang's session servers. Turn off only behind a proxy."},
	{Key: "enable-status", Kind: KindBool, Default: "true", Section: SecServer,
		Desc: "Answer server list pings. Disabling hides the server from the list."},
	{Key: "hide-online-players", Kind: KindBool, Default: "false", Section: SecServer,
		Desc: "Omit the player sample from the server list response."},
	{Key: "enforce-secure-profile", Kind: KindBool, Default: "true", Section: SecServer,
		Desc: "Require signed chat. Players without a Mojang signature cannot join."},
	{Key: "prevent-proxy-connections", Kind: KindBool, Default: "false", Section: SecServer,
		Desc: "Kick players whose authentication came from a different IP than the connection."},
	{Key: "accepts-transfers", Kind: KindBool, Default: "false", Section: SecServer,
		Desc: "Accept players transferred from another server by the transfer packet."},
	{Key: "bug-report-link", Kind: KindString, Default: "", Section: SecServer,
		Desc: "URL offered to players in the disconnect screen."},

	{Key: "gamemode", Kind: KindEnum, Default: "survival", Section: SecGameplay,
		Values: []string{"survival", "creative", "adventure", "spectator"},
		Desc:   "Mode new players start in."},
	{Key: "force-gamemode", Kind: KindBool, Default: "false", Section: SecGameplay,
		Desc: "Reset players to the default gamemode every time they join."},
	{Key: "difficulty", Kind: KindEnum, Default: "easy", Section: SecGameplay,
		Values: []string{"peaceful", "easy", "normal", "hard"},
		Desc:   "World difficulty. Peaceful removes hostile mobs entirely."},
	{Key: "hardcore", Kind: KindBool, Default: "false", Section: SecGameplay,
		Desc: "Death is permanent: players are switched to spectator instead of respawning."},
	{Key: "pvp", Kind: KindBool, Default: "true", Section: SecGameplay,
		Desc: "Allow players to damage each other."},
	{Key: "allow-flight", Kind: KindBool, Default: "false", Section: SecGameplay,
		Desc: "Tolerate flight in survival. Required by most movement plugins and mods."},
	{Key: "allow-nether", Kind: KindBool, Default: "true", Section: SecGameplay,
		Desc: "Enable nether portals and the nether dimension."},
	{Key: "spawn-monsters", Kind: KindBool, Default: "true", Section: SecGameplay,
		Desc: "Spawn hostile mobs."},
	{Key: "spawn-animals", Kind: KindBool, Default: "true", Section: SecGameplay,
		Desc: "Spawn passive mobs. Ignored on newer versions that moved this to the gamerule."},
	{Key: "spawn-npcs", Kind: KindBool, Default: "true", Section: SecGameplay,
		Desc: "Spawn villagers."},
	{Key: "enable-command-block", Kind: KindBool, Default: "false", Section: SecGameplay,
		Desc: "Allow command blocks to run."},

	{Key: "level-name", Kind: KindString, Default: "world", Section: SecWorld,
		Desc: "Directory holding the world. Changing it starts a fresh world."},
	{Key: "level-seed", Kind: KindString, Default: "", Section: SecWorld,
		Desc: "Seed for world generation. Only used when the world is first created."},
	{Key: "level-type", Kind: KindEnum, Default: "minecraft:normal", Section: SecWorld,
		Values: []string{"minecraft:normal", "minecraft:flat", "minecraft:large_biomes",
			"minecraft:amplified", "minecraft:single_biome_surface"},
		Desc: "World generator preset."},
	{Key: "generate-structures", Kind: KindBool, Default: "true", Section: SecWorld,
		Desc: "Generate villages, strongholds and other structures."},
	{Key: "generator-settings", Kind: KindString, Default: "{}", Section: SecWorld,
		Desc: "JSON customising the generator, mainly for superflat layers."},
	{Key: "max-world-size", Kind: KindInt, Default: "29999984", Section: SecWorld, Min: 1, Max: 29999984,
		Desc: "World border radius in blocks."},
	{Key: "spawn-protection", Kind: KindInt, Default: "16", Section: SecWorld, Min: 0, Max: 1000,
		Desc: "Radius around spawn only operators may build in. 0 disables it."},

	{Key: "white-list", Kind: KindBool, Default: "false", Section: SecPlayers,
		Desc: "Only players on whitelist.json may join."},
	{Key: "enforce-whitelist", Kind: KindBool, Default: "false", Section: SecPlayers,
		Desc: "Kick players already online when they are removed from the whitelist."},
	{Key: "op-permission-level", Kind: KindInt, Default: "4", Section: SecPlayers, Min: 1, Max: 4,
		Desc: "Permission level granted by /op. 4 allows every command."},
	{Key: "function-permission-level", Kind: KindInt, Default: "2", Section: SecPlayers, Min: 1, Max: 4,
		Desc: "Permission level datapack functions run at."},
	{Key: "player-idle-timeout", Kind: KindInt, Default: "0", Section: SecPlayers, Min: 0, Max: 525600,
		Desc: "Minutes before an idle player is kicked. 0 never kicks."},
	{Key: "broadcast-console-to-ops", Kind: KindBool, Default: "true", Section: SecPlayers,
		Desc: "Show commands run from the console to online operators."},
	{Key: "broadcast-rcon-to-ops", Kind: KindBool, Default: "true", Section: SecPlayers,
		Desc: "Show commands run over RCON to online operators."},
	{Key: "log-ips", Kind: KindBool, Default: "true", Section: SecPlayers,
		Desc: "Write player IP addresses to the log on join."},

	{Key: "view-distance", Kind: KindInt, Default: "10", Section: SecPerf, Min: 2, Max: 32,
		Desc: "Chunk radius sent to clients. The single biggest lever on CPU and bandwidth."},
	{Key: "simulation-distance", Kind: KindInt, Default: "10", Section: SecPerf, Min: 3, Max: 32,
		Desc: "Chunk radius where entities tick and crops grow."},
	{Key: "entity-broadcast-range-percentage", Kind: KindInt, Default: "100", Section: SecPerf, Min: 10, Max: 1000,
		Desc: "Percentage of the normal distance at which entities are sent to clients."},
	{Key: "network-compression-threshold", Kind: KindInt, Default: "256", Section: SecPerf, Min: -1, Max: 1000000,
		Desc: "Compress packets above this size. -1 disables compression."},
	{Key: "max-tick-time", Kind: KindInt, Default: "60000", Section: SecPerf, Min: -1, Max: 6000000,
		Desc: "Milliseconds a single tick may take before the watchdog kills the server. -1 disables it."},
	{Key: "pause-when-empty-seconds", Kind: KindInt, Default: "60", Section: SecPerf, Min: 0, Max: 1000000,
		Desc: "Stop ticking the world after this many seconds with no players. 0 keeps it running."},
	{Key: "sync-chunk-writes", Kind: KindBool, Default: "true", Section: SecPerf,
		Desc: "Flush chunk writes synchronously. Turning it off is faster but riskier on crash."},
	{Key: "use-native-transport", Kind: KindBool, Default: "true", Section: SecPerf,
		Desc: "Use optimised Linux packet handling."},
	{Key: "max-chained-neighbor-updates", Kind: KindInt, Default: "1000000", Section: SecPerf, Min: -1, Max: 100000000,
		Desc: "Limit on chained block updates before the rest are skipped."},

	{Key: "resource-pack", Kind: KindString, Default: "", Section: SecPack,
		Desc: "URL of a resource pack offered on join."},
	{Key: "resource-pack-sha1", Kind: KindString, Default: "", Section: SecPack,
		Desc: "SHA-1 of the pack. Clients re-download the pack when it does not match."},
	{Key: "resource-pack-prompt", Kind: KindString, Default: "", Section: SecPack,
		Desc: "JSON text component shown in the pack confirmation dialog."},
	{Key: "resource-pack-id", Kind: KindString, Default: "", Section: SecPack,
		Desc: "UUID identifying the pack."},
	{Key: "require-resource-pack", Kind: KindBool, Default: "false", Section: SecPack,
		Desc: "Disconnect players who decline the resource pack."},

	{Key: "enable-rcon", Kind: KindBool, Default: "false", Section: SecRemote,
		Desc: "Expose the remote console protocol."},
	{Key: "rcon.port", Kind: KindInt, Default: "25575", Section: SecRemote, Min: 1, Max: 65535,
		Desc: "Port the RCON listener binds to."},
	{Key: "rcon.password", Kind: KindString, Default: "", Section: SecRemote, Secret: true,
		Desc: "RCON password. Anyone holding it has full console access."},
	{Key: "enable-query", Kind: KindBool, Default: "false", Section: SecRemote,
		Desc: "Expose the GameSpy4 query protocol used by server list sites."},
	{Key: "query.port", Kind: KindInt, Default: "25565", Section: SecRemote, Min: 1, Max: 65535,
		Desc: "Port the query listener binds to."},

	{Key: "enable-jmx-monitoring", Kind: KindBool, Default: "false", Section: SecAdvanced,
		Desc: "Expose tick timings over JMX."},
	{Key: "rate-limit", Kind: KindInt, Default: "0", Section: SecAdvanced, Min: 0, Max: 1000000,
		Desc: "Packets per second before a client is kicked. 0 disables the limit."},
	{Key: "initial-enabled-packs", Kind: KindString, Default: "vanilla", Section: SecAdvanced,
		Desc: "Comma separated datapacks enabled when the world is created."},
	{Key: "initial-disabled-packs", Kind: KindString, Default: "", Section: SecAdvanced,
		Desc: "Comma separated datapacks disabled when the world is created."},
	{Key: "region-file-compression", Kind: KindEnum, Default: "deflate", Section: SecAdvanced,
		Values: []string{"deflate", "lz4", "none"},
		Desc:   "Compression used for region files. lz4 is faster but larger."},
	{Key: "text-filtering-config", Kind: KindString, Default: "", Section: SecAdvanced,
		Desc: "Endpoint of an external chat filtering service."},
	{Key: "text-filtering-version", Kind: KindInt, Default: "0", Section: SecAdvanced, Min: 0, Max: 100,
		Desc: "Protocol version used against the chat filtering service."},
}

var catalogIndex = func() map[string]Spec {
	m := make(map[string]Spec, len(Catalog))
	for _, s := range Catalog {
		m[s.Key] = s
	}
	return m
}()

func Lookup(key string) (Spec, bool) {
	s, ok := catalogIndex[key]
	return s, ok
}

// SpecFor always returns a usable spec: unknown keys are treated as free-form
// strings so a server with modded properties still renders.
func SpecFor(key string) Spec {
	if s, ok := catalogIndex[key]; ok {
		return s
	}
	return Spec{Key: key, Kind: KindString, Section: SecOther}
}

func SectionRank(section string) int {
	for i, s := range Sections {
		if s == section {
			return i
		}
	}
	return len(Sections)
}
