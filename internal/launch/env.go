package launch

import (
	"runtime"
	"strconv"

	"github.com/bx-team/irori/internal/java"
)

type Env struct {
	HeapMB int
	Major  int
	Graal  java.Graal
	GOOS   string
	GOARCH string
}

func EnvFor(jdk java.JDK, heapMB int) Env {
	return Env{
		HeapMB: heapMB,
		Major:  jdk.Major,
		Graal:  jdk.Graal,
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
	}
}

func (e Env) since(major int) bool  { return e.Major >= major }
func (e Env) before(major int) bool { return e.Major > 0 && e.Major < major }

func (e Env) x86() bool   { return e.GOARCH == "amd64" || e.GOARCH == "386" }
func (e Env) linux() bool { return e.GOOS == "linux" }
func (e Env) unix() bool  { return e.GOOS != "windows" }

func (e Env) graalEE() bool { return e.Graal == java.GraalOracle }

func (e Env) graalProp(name, value string) string {
	if e.since(23) {
		return "-Djdk.graal." + name + "=" + value
	}
	return "-Dgraal." + name + "=" + value
}

// vectorModule is only on Java 17+
func (e Env) vectorModule() []string {
	if !e.since(17) {
		return nil
	}
	return []string{"--add-modules=jdk.incubator.vector"}
}

type builder struct {
	env  Env
	args []string
}

func newBuilder(e Env) *builder {
	if e.GOOS == "" {
		e.GOOS = runtime.GOOS
	}
	if e.GOARCH == "" {
		e.GOARCH = runtime.GOARCH
	}
	return &builder{env: e}
}

func (b *builder) add(args ...string) *builder {
	b.args = append(b.args, args...)
	return b
}

func (b *builder) when(cond bool, args ...string) {
	if cond {
		b.args = append(b.args, args...)
	}
}

func (b *builder) mb(flag string, value int) {
	b.add(flag + strconv.Itoa(value) + "M")
}

func (b *builder) done() []string { return b.args }
