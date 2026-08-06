package irori_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bx-team/irori/internal/importer"
	"github.com/bx-team/irori/internal/java"
	"github.com/bx-team/irori/internal/launch"
)

func flagsFor(id string, e launch.Env) []string {
	return launch.GetPreset(id).Flags(e)
}

func has(flags []string, want string) bool {
	return slices.Contains(flags, want)
}

func hasPrefix(flags []string, prefix string) bool {
	for _, f := range flags {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// Every boundary here was verified against a real JDK 17, 21 and 25.
func TestVersionGatedFlagsStayOnTheirSideOfTheBoundary(t *testing.T) {
	cases := []struct {
		flag     string
		preset   string
		okMajor  int
		badMajor int
	}{
		{"-XX:G1ConcRSHotCardLimit=16", "obydux", 21, 25},
		{"-XX:G1ConcRefinementServiceIntervalMillis=150", "bruce", 21, 25},
		{"-XX:ShenandoahGCMode=iu", "hilltty", 21, 25},
		{"-XX:-UseBiasedLocking", "etil", 17, 21},
		{"-XX:+UseCompactObjectHeaders", "meowice", 25, 21},
		{"--add-modules=jdk.incubator.vector", "meowice", 17, 11},
	}
	for _, c := range cases {
		ok := flagsFor(c.preset, launch.Env{HeapMB: 8192, Major: c.okMajor, GOOS: "linux", GOARCH: "amd64"})
		if !has(ok, c.flag) {
			t.Errorf("%s: expected %s on Java %d", c.preset, c.flag, c.okMajor)
		}
		bad := flagsFor(c.preset, launch.Env{HeapMB: 8192, Major: c.badMajor, GOOS: "linux", GOARCH: "amd64"})
		if has(bad, c.flag) {
			t.Errorf("%s: %s must not be emitted on Java %d, that JVM will not start", c.preset, c.flag, c.badMajor)
		}
	}
}

// Guessing high breaks a Java 17 box, guessing low breaks a Java 25 one.
func TestUnknownJavaVersionGetsNoGatedFlags(t *testing.T) {
	e := launch.Env{HeapMB: 8192, GOOS: "linux", GOARCH: "amd64"}
	for _, id := range []string{"meowice", "obydux", "bruce", "hilltty", "etil"} {
		for _, f := range flagsFor(id, e) {
			switch f {
			case "-XX:G1ConcRSHotCardLimit=16",
				"-XX:ShenandoahGCMode=iu",
				"-XX:-UseBiasedLocking",
				"-XX:+UseCompactObjectHeaders",
				"--add-modules=jdk.incubator.vector":
				t.Errorf("%s emits %s for an unknown Java version", id, f)
			}
		}
	}
}

func TestGraalOptionsOnlyReachOracleGraalVM(t *testing.T) {
	for _, id := range []string{"meowice", "meowice-zgc", "obydux", "bruce"} {
		for _, g := range []java.Graal{java.GraalNone, java.GraalCommunity} {
			e := launch.Env{HeapMB: 8192, Major: 25, Graal: g, GOOS: "linux", GOARCH: "amd64"}
			if hasPrefix(flagsFor(id, e), "-Djdk.graal.") || hasPrefix(flagsFor(id, e), "-Dgraal.") {
				t.Errorf("%s leaks a Graal compiler option to graal=%q", id, g)
			}
		}
		e := launch.Env{HeapMB: 8192, Major: 25, Graal: java.GraalOracle, GOOS: "linux", GOARCH: "amd64"}
		if !hasPrefix(flagsFor(id, e), "-Djdk.graal.") {
			t.Errorf("%s emits no Graal compiler option on Oracle GraalVM", id)
		}
	}
}

// GraalVM renamed the prefix in its JDK 23 release and dropped the old one.
func TestGraalPropertyPrefixFollowsTheRuntime(t *testing.T) {
	old := flagsFor("obydux", launch.Env{HeapMB: 8192, Major: 21, Graal: java.GraalOracle, GOOS: "linux", GOARCH: "amd64"})
	if !has(old, "-Dgraal.CompilerConfiguration=enterprise") {
		t.Error("Java 21 GraalVM must use the -Dgraal. prefix")
	}
	current := flagsFor("obydux", launch.Env{HeapMB: 8192, Major: 25, Graal: java.GraalOracle, GOOS: "linux", GOARCH: "amd64"})
	if !has(current, "-Djdk.graal.CompilerConfiguration=enterprise") {
		t.Error("Java 25 GraalVM must use the -Djdk.graal. prefix")
	}
}

func TestX86IntrinsicsNeverReachAnArmJVM(t *testing.T) {
	x86Only := []string{
		"-XX:+UseFastStosb", "-XX:+UseXmmI2D", "-XX:+UseXmmI2F",
		"-XX:+UseXmmLoadAndClearUpper", "-XX:+UseXmmRegToRegMoveAll",
		"-XX:+UseXMMForArrayCopy", "-XX:+UseNewLongLShift", "-XX:+UseVectorCmov",
		"-XX:+UseFPUForSpilling", "-XX:+UseVectorStubs", "-XX:UseAVX=3", "-XX:UseSSE=4",
	}
	for _, id := range []string{"meowice", "meowice-zgc", "bruce", "etil"} {
		arm := flagsFor(id, launch.Env{HeapMB: 8192, Major: 25, GOOS: "linux", GOARCH: "arm64"})
		for _, f := range x86Only {
			if has(arm, f) {
				t.Errorf("%s emits the x86-only %s on arm64", id, f)
			}
		}
	}
}

// "The unlock option must precede <flag>": the JVM enforces the order, not
// merely the presence.
func TestUnlockOptionsComeBeforeWhatTheyUnlock(t *testing.T) {
	guarded := []string{
		"-XX:+UseCompactObjectHeaders", "-XX:+EnableVectorSupport",
		"-XX:+UseCharacterCompareIntrinsics", "-XX:+TrustFinalNonStaticFields",
		"-XX:+UseFastUnorderedTimeStamps", "-XX:+UseCriticalJavaThreadPriority",
		"-XX:+UseVectorStubs", "-XX:ShenandoahGCMode=iu", "-XX:-ZProactive",
	}
	for _, p := range launch.Presets() {
		for _, major := range []int{17, 21, 25} {
			flags := p.Flags(launch.Env{HeapMB: 8192, Major: major, GOOS: "linux", GOARCH: "amd64"})
			lastUnlock, firstGuarded := -1, len(flags)
			for i, f := range flags {
				switch {
				case f == "-XX:+UnlockExperimentalVMOptions", f == "-XX:+UnlockDiagnosticVMOptions":
					lastUnlock = i
				case slices.Contains(guarded, f) && i < firstGuarded:
					firstGuarded = i
				}
			}
			if firstGuarded < len(flags) && lastUnlock > firstGuarded {
				t.Errorf("%s on Java %d: %s at %d is unlocked only at %d",
					p.ID, major, flags[firstGuarded], firstGuarded, lastUnlock)
			}
		}
	}
}

// A missed match silently downgrades the import to "none" and dumps the whole
// set into extraFlags. The lines are verbatim from the upstream READMEs.
func TestImportRecognisesPresetsWrittenForAnotherJVM(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"aikar", "java -Xms10G -Xmx10G -XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=200 -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:+AlwaysPreTouch -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=40 -XX:G1HeapRegionSize=8M -XX:G1ReservePercent=20 -XX:G1HeapWastePercent=5 -XX:G1MixedGCCountTarget=4 -XX:InitiatingHeapOccupancyPercent=15 -XX:G1MixedGCLiveThresholdPercent=90 -XX:G1RSetUpdatingPauseTimePercent=5 -XX:SurvivorRatio=32 -XX:+PerfDisableSharedMem -XX:MaxTenuringThreshold=1 -Dusing.aikars.flags=https://mcflags.emc.gs -Daikars.new.flags=true -jar paper.jar --nogui", "aikar"},
		{"meowice", "java -Xms6G -Xmx6G --add-modules=jdk.incubator.vector -XX:+UseG1GC -XX:MaxGCPauseMillis=200 -XX:+UnlockExperimentalVMOptions -XX:+UnlockDiagnosticVMOptions -XX:+DisableExplicitGC -XX:+AlwaysPreTouch -XX:G1NewSizePercent=28 -XX:G1MaxNewSizePercent=50 -XX:G1HeapRegionSize=16M -XX:G1ReservePercent=15 -XX:G1MixedGCCountTarget=3 -XX:InitiatingHeapOccupancyPercent=20 -XX:G1MixedGCLiveThresholdPercent=90 -XX:SurvivorRatio=32 -XX:G1HeapWastePercent=5 -XX:+PerfDisableSharedMem -XX:G1SATBBufferEnqueueingThresholdPercent=30 -XX:G1ConcMarkStepDurationMillis=5 -XX:G1RSetUpdatingPauseTimePercent=0 -XX:+UseNUMA -XX:-DontCompileHugeMethods -XX:MaxNodeLimit=240000 -XX:NodeLimitFudgeFactor=8000 -XX:ReservedCodeCacheSize=400M -XX:NonNMethodCodeHeapSize=12M -XX:ProfiledCodeHeapSize=194M -XX:NonProfiledCodeHeapSize=194M -XX:NmethodSweepActivity=1 -XX:+UseCriticalJavaThreadPriority -XX:AllocatePrefetchStyle=3 -XX:+AlwaysActAsServerClassMachine -XX:+UseTransparentHugePages -XX:LargePageSizeInBytes=2M -XX:+UseLargePages -XX:+EagerJVMCI -XX:+UseStringDeduplication -XX:+UseAES -XX:+UseAESIntrinsics -XX:+UseFMA -XX:+UseLoopPredicate -XX:+RangeCheckElimination -XX:+OptimizeStringConcat -XX:+UseCompressedOops -XX:+UseThreadPriorities -XX:+OmitStackTraceInFastThrow -XX:+RewriteBytecodes -XX:+RewriteFrequentPairs -XX:+UseFPUForSpilling -XX:+UseFastStosb -XX:+UseNewLongLShift -XX:+UseVectorCmov -XX:+UseXMMForArrayCopy -XX:+UseXmmI2D -XX:+UseXmmI2F -XX:+UseXmmLoadAndClearUpper -XX:+UseXmmRegToRegMoveAll -XX:+EliminateLocks -XX:+DoEscapeAnalysis -XX:+AlignVector -XX:+OptimizeFill -XX:+EnableVectorSupport -XX:+UseCharacterCompareIntrinsics -XX:+UseCopySignIntrinsic -XX:+UseVectorStubs -XX:+UseFastJNIAccessors -XX:+UseInlineCaches -XX:+SegmentedCodeCache -XX:+UseCompactObjectHeaders -Djdk.nio.maxCachedBufferSize=262144 -jar server.jar nogui", "meowice"},
		{"plain", "java -Xms2G -Xmx2G -jar server.jar nogui", "none"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "start.sh")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+c.line+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		s, ok := importer.ParseStartScript(path)
		if !ok {
			t.Fatalf("%s: start script not parsed", c.name)
		}
		if s.Preset != c.want {
			t.Errorf("%s: detected preset %q, want %q", c.name, s.Preset, c.want)
		}
	}
}

// The one test that would have caught every gate at once, given a JDK to run.
func TestPresetsStartTheLocalJVM(t *testing.T) {
	bin, err := exec.LookPath("java")
	if err != nil {
		t.Skip("no java on PATH")
	}
	jdk, err := java.Probe(t.Context(), bin)
	if err != nil {
		t.Skipf("could not probe %s: %v", bin, err)
	}

	for _, p := range launch.Presets() {
		args := append([]string{"-Xms1G", "-Xmx1G"}, p.Flags(launch.EnvFor(jdk, 1024))...)
		out, err := exec.Command(bin, append(args, "-version")...).CombinedOutput()
		if err != nil {
			t.Errorf("preset %s does not start Java %d:\n%s", p.ID, jdk.Major, out)
		}
	}
}
