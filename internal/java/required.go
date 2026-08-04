package java

import "strconv"

// requirements is the minimum JDK each Minecraft release needs, as a set of
// floors: a version takes the java of the newest entry it is greater than or
// equal to. Mojang has raised the floor exactly five times, so a table beats a
// per-version map that would need editing every fortnight.
//
// This is only a fallback. The authoritative number comes from mcjars and is
// stored as java.major in .irori.json; RequiredFor is what keeps a server that
// was adopted from an existing directory — or configured before irori recorded
// the field — from falling back to "any JDK will do".
var requirements = []struct {
	from string
	java int
}{
	{"1.0", 8},
	{"1.17", 16},
	{"1.18", 17},
	{"1.20.5", 21},
	{"26", 25},
}

// RequiredFor guesses the JDK a Minecraft version needs from its number alone.
// It returns 0 for anything it cannot parse, which callers must read as "no
// requirement" rather than "Java 0".
func RequiredFor(mcVersion string) int {
	parts := versionParts(mcVersion)
	if len(parts) == 0 {
		return 0
	}
	major := 0
	for _, r := range requirements {
		if compareVersions(parts, versionParts(r.from)) >= 0 {
			major = r.java
		}
	}
	return major
}

func versionParts(v string) []int {
	var out []int
	cur, digits := 0, false
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= '0' && c <= '9':
			cur = cur*10 + int(c-'0')
			digits = true
		case c == '.' && digits:
			out = append(out, cur)
			cur, digits = 0, false
		default:
			if digits {
				out = append(out, cur)
			}
			return out
		}
	}
	if digits {
		out = append(out, cur)
	}
	return out
}

func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func RequirementNote(explicit int, mcVersion string) string {
	if explicit > 0 {
		return "Java " + strconv.Itoa(explicit) + "+ required"
	}
	if guess := RequiredFor(mcVersion); guess > 0 {
		return "Java " + strconv.Itoa(guess) + "+ required (guessed from " + mcVersion + ")"
	}
	return ""
}
