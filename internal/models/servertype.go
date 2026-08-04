package models

import "strings"

type ServerType string

const (
	TypeVanilla      ServerType = "VANILLA"
	TypePaper        ServerType = "PAPER"
	TypePufferfish   ServerType = "PUFFERFISH"
	TypeSpigot       ServerType = "SPIGOT"
	TypeFolia        ServerType = "FOLIA"
	TypePurpur       ServerType = "PURPUR"
	TypeWaterfall    ServerType = "WATERFALL"
	TypeVelocity     ServerType = "VELOCITY"
	TypeFabric       ServerType = "FABRIC"
	TypeBungeeCord   ServerType = "BUNGEECORD"
	TypeQuilt        ServerType = "QUILT"
	TypeForge        ServerType = "FORGE"
	TypeNeoForge     ServerType = "NEOFORGE"
	TypeMohist       ServerType = "MOHIST"
	TypeArclight     ServerType = "ARCLIGHT"
	TypeSponge       ServerType = "SPONGE"
	TypeLeaves       ServerType = "LEAVES"
	TypeCanvas       ServerType = "CANVAS"
	TypeASPaper      ServerType = "ASPAPER"
	TypeLegacyFabric ServerType = "LEGACY_FABRIC"
	TypeLoohpLimbo   ServerType = "LOOHP_LIMBO"
	TypeNanoLimbo    ServerType = "NANOLIMBO"
	TypeVelocityCTD  ServerType = "VELOCITY_CTD"
	TypeDivineMC     ServerType = "DIVINEMC"
	TypeMagma        ServerType = "MAGMA"
	TypeLeaf         ServerType = "LEAF"
	TypeYouer        ServerType = "YOUER"
	TypePluto        ServerType = "PLUTO"
)

func (t ServerType) Normalize() ServerType {
	return ServerType(strings.ToUpper(strings.TrimSpace(string(t))))
}

func (t ServerType) Display() string {
	switch t.Normalize() {
	case TypeNeoForge:
		return "NeoForge"
	case TypeLegacyFabric:
		return "Legacy Fabric"
	case TypeBungeeCord:
		return "BungeeCord"
	case TypeDivineMC:
		return "DivineMC"
	case TypeASPaper:
		return "ASPaper"
	}
	s := strings.ToLower(string(t.Normalize()))
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// IsProxy reports whether the type is a proxy rather than a game server:
// proxies have no world, no server.properties and their own config layout.
func (t ServerType) IsProxy() bool {
	switch t.Normalize() {
	case TypeVelocity, TypeVelocityCTD, TypeWaterfall, TypeBungeeCord:
		return true
	}
	return false
}

// IsLimbo reports a minimal standalone server: no plugins, no server.properties.
func (t ServerType) IsLimbo() bool {
	switch t.Normalize() {
	case TypeLoohpLimbo, TypeNanoLimbo:
		return true
	}
	return false
}

// AddonDir is the directory server addons are dropped into: mod loaders use
// mods/, everything else uses plugins/.
func (t ServerType) AddonDir() string {
	switch t.Normalize() {
	case TypeFabric, TypeQuilt, TypeForge, TypeNeoForge, TypeLegacyFabric:
		return "mods"
	}
	return "plugins"
}

func (t ServerType) AddonNoun() string {
	if t.AddonDir() == "mods" {
		return "mod"
	}
	return "plugin"
}

// ModrinthLoaders maps a server type onto the loader facets Modrinth indexes
// addons under. Paper-family servers accept bukkit and spigot jars too.
func (t ServerType) ModrinthLoaders() []string {
	switch t.Normalize() {
	case TypePaper, TypePurpur, TypePufferfish, TypeLeaves, TypeCanvas,
		TypeASPaper, TypeDivineMC, TypeLeaf, TypeYouer, TypePluto:
		return []string{"paper", "spigot", "bukkit"}
	case TypeFolia:
		return []string{"folia", "paper", "bukkit"}
	case TypeSpigot:
		return []string{"spigot", "bukkit"}
	case TypeMohist, TypeArclight, TypeMagma:
		return []string{"paper", "spigot", "bukkit", "forge"}
	case TypeFabric:
		return []string{"fabric"}
	case TypeLegacyFabric:
		return []string{"fabric", "legacy-fabric"}
	case TypeQuilt:
		return []string{"quilt", "fabric"}
	case TypeForge:
		return []string{"forge"}
	case TypeNeoForge:
		return []string{"neoforge", "forge"}
	case TypeVelocity, TypeVelocityCTD:
		return []string{"velocity"}
	case TypeWaterfall, TypeBungeeCord:
		return []string{"bungeecord", "waterfall"}
	case TypeSponge:
		return []string{"sponge"}
	default:
		return nil
	}
}

// ModrinthProjectTypes narrows search to plugins or mods depending on loader family.
func (t ServerType) ModrinthProjectTypes() []string {
	if t.AddonDir() == "mods" {
		return []string{"mod"}
	}
	return []string{"plugin"}
}
