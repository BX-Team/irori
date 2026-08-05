//go:build !windows

package java

const javaBin = "java"

var systemGlobs = []string{
	"/usr/lib/jvm/*/bin/java",
	"/usr/java/*/bin/java",
	"/opt/java/*/bin/java",
}
