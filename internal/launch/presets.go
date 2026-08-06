package launch

import (
	"sort"

	"github.com/bx-team/irori/internal/java"
)

type Preset struct {
	ID      string
	Name    string
	Summary string
	Flags   func(Env) []string
}

var presets = map[string]Preset{
	"none": {
		ID:      "none",
		Name:    "No flags",
		Summary: "Only -Xms/-Xmx, everything else left to the JVM",
		Flags:   func(Env) []string { return nil },
	},
	"aikar": {
		ID:      "aikar",
		Name:    "Aikar's Flags",
		Summary: "G1GC tuning, the long-standing default for Paper and its forks",
		Flags:   aikarFlags,
	},
	"meowice": {
		ID:      "meowice",
		Name:    "MeowIce's Flags (G1GC)",
		Summary: "Aikar's G1GC plus modern JIT and intrinsic tuning; best on GraalVM",
		Flags:   meowiceG1Flags,
	},
	"meowice-zgc": {
		ID:      "meowice-zgc",
		Name:    "MeowIce's Flags (ZGC)",
		Summary: "The same set on ZGC — for 32G+ heaps on 10+ cores only",
		Flags:   meowiceZGCFlags,
	},
	"obydux": {
		ID:      "obydux",
		Name:    "Obydux's GraalVM Flags",
		Summary: "G1GC with GraalVM JIT options; Linux only, Java 17+",
		Flags:   obyduxFlags,
	},
	"bruce": {
		ID:      "bruce",
		Name:    "brucethemoose's Flags",
		Summary: "Individually benchmarked base flags with the server G1GC set",
		Flags:   bruceFlags,
	},
	"hilltty": {
		ID:      "hilltty",
		Name:    "hilltty's Flags",
		Summary: "Shenandoah GC, short pauses at the cost of CPU; a small set",
		Flags:   hillttyFlags,
	},
	"etil": {
		ID:      "etil",
		Name:    "etil's Flags",
		Summary: "Aikar's G1GC with extra intrinsics; unmaintained, kept for parity",
		Flags:   etilFlags,
	},
}

func aikarFlags(e Env) []string {
	newSize, maxNewSize, regionSize, reserve, ihop := "30", "40", "8M", "20", "15"
	if e.HeapMB >= 12*1024 {
		newSize, maxNewSize, regionSize, reserve, ihop = "40", "50", "16M", "15", "20"
	}
	return newBuilder(e).add(
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent="+newSize,
		"-XX:G1MaxNewSizePercent="+maxNewSize,
		"-XX:G1HeapRegionSize="+regionSize,
		"-XX:G1ReservePercent="+reserve,
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent="+ihop,
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		"-Daikars.new.flags=true",
	).done()
}

func meowiceG1Flags(e Env) []string {
	b := newBuilder(e)
	b.add(e.vectorModule()...)
	unlock(b)
	b.add(
		"-XX:+UseG1GC",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=28",
		"-XX:G1MaxNewSizePercent=50",
		"-XX:G1HeapRegionSize=16M",
		"-XX:G1ReservePercent=15",
		"-XX:G1MixedGCCountTarget=3",
		"-XX:InitiatingHeapOccupancyPercent=20",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:SurvivorRatio=32",
		"-XX:G1HeapWastePercent=5",
		"-XX:+PerfDisableSharedMem",
		"-XX:G1SATBBufferEnqueueingThresholdPercent=30",
		"-XX:G1ConcMarkStepDurationMillis=5",
		"-XX:G1RSetUpdatingPauseTimePercent=0",
		"-XX:AllocatePrefetchStyle=3",
	)
	return meowiceCommon(b)
}

func meowiceZGCFlags(e Env) []string {
	b := newBuilder(e)
	b.add(e.vectorModule()...)
	unlock(b)
	b.add(
		"-XX:+UseZGC",
		"-XX:-ZProactive",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:+PerfDisableSharedMem",
	)
	if e.HeapMB > 2048 {
		b.mb("-XX:SoftMaxHeapSize=", e.HeapMB-2048)
	}
	b.add("-XX:AllocatePrefetchStyle=1")
	return meowiceCommon(b)
}

func meowiceCommon(b *builder) []string {
	e := b.env
	b.add(
		"-XX:+UseNUMA",
		"-XX:-DontCompileHugeMethods",
		"-XX:MaxNodeLimit=240000",
		"-XX:NodeLimitFudgeFactor=8000",
		"-XX:ReservedCodeCacheSize=400M",
		"-XX:NonNMethodCodeHeapSize=12M",
		"-XX:ProfiledCodeHeapSize=194M",
		"-XX:NonProfiledCodeHeapSize=194M",
		"-XX:NmethodSweepActivity=1",
		"-XX:+UseCriticalJavaThreadPriority",
		"-XX:+AlwaysActAsServerClassMachine",
	)
	largePages(b)
	b.add(
		"-XX:+EagerJVMCI",
		"-XX:+UseStringDeduplication",
		"-XX:+UseAES",
		"-XX:+UseAESIntrinsics",
		"-XX:+UseFMA",
		"-XX:+UseLoopPredicate",
		"-XX:+RangeCheckElimination",
		"-XX:+OptimizeStringConcat",
		"-XX:+UseCompressedOops",
		"-XX:+UseThreadPriorities",
		"-XX:+OmitStackTraceInFastThrow",
		"-XX:+RewriteBytecodes",
		"-XX:+RewriteFrequentPairs",
		"-XX:+EliminateLocks",
		"-XX:+DoEscapeAnalysis",
		"-XX:+AlignVector",
		"-XX:+OptimizeFill",
		"-XX:+EnableVectorSupport",
		"-XX:+UseCharacterCompareIntrinsics",
		"-XX:+UseCopySignIntrinsic",
		"-XX:+UseFastJNIAccessors",
		"-XX:+UseInlineCaches",
		"-XX:+SegmentedCodeCache",
	)
	x86Intrinsics(b)
	b.when(e.x86(), "-XX:+UseVectorStubs")
	// Compact object headers only exist from Java 24 on.
	b.when(e.since(24), "-XX:+UseCompactObjectHeaders")
	b.add("-Djdk.nio.maxCachedBufferSize=262144")
	b.when(e.graalEE(),
		e.graalProp("UsePriorityInlining", "true"),
		e.graalProp("Vectorization", "true"),
		e.graalProp("OptDuplication", "true"),
		e.graalProp("DetectInvertedLoopsAsCounted", "true"),
		e.graalProp("LoopInversion", "true"),
		e.graalProp("VectorizeHashes", "true"),
		e.graalProp("EnterprisePartialUnroll", "true"),
		e.graalProp("VectorizeSIMD", "true"),
		e.graalProp("StripMineNonCountedLoops", "true"),
		e.graalProp("SpeculativeGuardMovement", "true"),
		e.graalProp("TuneInlinerExploration", "1"),
		e.graalProp("LoopRotation", "true"),
		e.graalProp("CompilerConfiguration", "enterprise"),
	)
	return b.done()
}

func obyduxFlags(e Env) []string {
	b := newBuilder(e)
	b.add(e.vectorModule()...)
	unlock(b)
	b.add(
		"-XX:+UseG1GC",
		"-XX:MaxGCPauseMillis=130",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=28",
		"-XX:G1HeapRegionSize=16M",
		"-XX:G1ReservePercent=20",
		"-XX:G1MixedGCCountTarget=3",
		"-XX:InitiatingHeapOccupancyPercent=10",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:SurvivorRatio=32",
		"-XX:MaxTenuringThreshold=1",
		"-XX:+PerfDisableSharedMem",
		"-XX:G1SATBBufferEnqueueingThresholdPercent=30",
		"-XX:G1ConcMarkStepDurationMillis=5",
	)
	g1Refinement(b)
	b.add(
		"-XX:G1RSetUpdatingPauseTimePercent=0",
		"-XX:+UseNUMA",
		"-XX:-DontCompileHugeMethods",
		"-XX:MaxNodeLimit=240000",
		"-XX:NodeLimitFudgeFactor=8000",
		"-XX:ReservedCodeCacheSize=400M",
		"-XX:NonNMethodCodeHeapSize=12M",
		"-XX:ProfiledCodeHeapSize=194M",
		"-XX:NonProfiledCodeHeapSize=194M",
		"-XX:NmethodSweepActivity=1",
		"-XX:+UseFastUnorderedTimeStamps",
		"-XX:+UseCriticalJavaThreadPriority",
		"-XX:AllocatePrefetchStyle=3",
		"-XX:+AlwaysActAsServerClassMachine",
	)
	largePages(b)
	b.add("-XX:+EagerJVMCI")
	b.when(e.graalEE(),
		e.graalProp("TuneInlinerExploration", "1"),
		e.graalProp("LoopRotation", "true"),
		e.graalProp("OptWriteMotion", "true"),
		e.graalProp("CompilerConfiguration", "enterprise"),
	)
	return b.done()
}

func bruceFlags(e Env) []string {
	b := newBuilder(e)
	unlock(b)
	b.add(
		"-XX:+AlwaysActAsServerClassMachine",
		"-XX:+AlwaysPreTouch",
		"-XX:+DisableExplicitGC",
		"-XX:+UseNUMA",
		"-XX:NmethodSweepActivity=1",
		"-XX:ReservedCodeCacheSize=400M",
		"-XX:NonNMethodCodeHeapSize=12M",
		"-XX:ProfiledCodeHeapSize=194M",
		"-XX:NonProfiledCodeHeapSize=194M",
		"-XX:-DontCompileHugeMethods",
		"-XX:MaxNodeLimit=240000",
		"-XX:NodeLimitFudgeFactor=8000",
		"-XX:+PerfDisableSharedMem",
		"-XX:+UseFastUnorderedTimeStamps",
		"-XX:+UseCriticalJavaThreadPriority",
		"-XX:ThreadPriorityPolicy=1",
		"-XX:AllocatePrefetchStyle=3",
	)
	b.when(e.x86(), "-XX:+UseVectorCmov")
	b.add(
		"-XX:+UseG1GC",
		"-XX:MaxGCPauseMillis=130",
		"-XX:G1NewSizePercent=28",
		"-XX:G1HeapRegionSize=16M",
		"-XX:G1ReservePercent=20",
		"-XX:G1MixedGCCountTarget=3",
		"-XX:InitiatingHeapOccupancyPercent=10",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=0",
		"-XX:SurvivorRatio=32",
		"-XX:MaxTenuringThreshold=1",
		"-XX:G1SATBBufferEnqueueingThresholdPercent=30",
		"-XX:G1ConcMarkStepDurationMillis=5",
	)
	g1Refinement(b)
	largePages(b)
	b.when(e.graalEE(),
		"-XX:+EagerJVMCI",
		e.graalProp("TuneInlinerExploration", "1"),
		e.graalProp("CompilerConfiguration", "enterprise"),
	)
	return b.done()
}

func hillttyFlags(e Env) []string {
	b := newBuilder(e)
	b.add("-XX:+UnlockExperimentalVMOptions")
	largePages(b)
	b.add("-XX:+UseShenandoahGC")
	// Shenandoah's incremental-update mode was removed in Java 24
	b.when(e.before(24), "-XX:ShenandoahGCMode=iu")
	b.add(
		"-XX:+UseNUMA",
		"-XX:+AlwaysPreTouch",
		"-XX:+DisableExplicitGC",
	)
	biasedLocking(b)
	return b.add("-Dfile.encoding=UTF-8").done()
}

func etilFlags(e Env) []string {
	newSize, maxNewSize, regionSize, reserve, ihop := "30", "40", "8M", "20", "15"
	if e.HeapMB > 12*1024 {
		newSize, maxNewSize, regionSize, reserve, ihop = "40", "50", "16M", "15", "20"
	}
	b := newBuilder(e)
	b.add(e.vectorModule()...)
	unlock(b)
	b.add(
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent="+newSize,
		"-XX:G1MaxNewSizePercent="+maxNewSize,
		"-XX:G1HeapRegionSize="+regionSize,
		"-XX:G1ReservePercent="+reserve,
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent="+ihop,
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
	)
	biasedLocking(b)
	b.when(e.x86(), "-XX:UseAVX=3", "-XX:UseSSE=4")
	b.add(
		"-XX:+UseStringDeduplication",
		"-XX:+UseFastUnorderedTimeStamps",
		"-XX:+UseAES",
		"-XX:+UseAESIntrinsics",
		"-XX:+UseFMA",
		"-XX:AllocatePrefetchStyle=1",
		"-XX:+UseLoopPredicate",
		"-XX:+RangeCheckElimination",
		"-XX:+EliminateLocks",
		"-XX:+DoEscapeAnalysis",
		"-XX:+UseCodeCacheFlushing",
		"-XX:+SegmentedCodeCache",
		"-XX:+UseFastJNIAccessors",
		"-XX:+OptimizeStringConcat",
		"-XX:+UseCompressedOops",
		"-XX:+UseThreadPriorities",
		"-XX:+OmitStackTraceInFastThrow",
		"-XX:+TrustFinalNonStaticFields",
		"-XX:ThreadPriorityPolicy=1",
		"-XX:+UseInlineCaches",
		"-XX:+RewriteBytecodes",
		"-XX:+RewriteFrequentPairs",
		"-XX:+UseNUMA",
		"-XX:-DontCompileHugeMethods",
	)
	x86Intrinsics(b)
	b.add("-Dfile.encoding=UTF-8")
	b.when(e.since(17), "-Xlog:async")
	b.when(e.unix(), "-Djava.security.egd=file:/dev/urandom")
	return b.done()
}

func unlock(b *builder) {
	b.add("-XX:+UnlockExperimentalVMOptions", "-XX:+UnlockDiagnosticVMOptions")
}

func largePages(b *builder) {
	b.when(b.env.linux(), "-XX:+UseTransparentHugePages")
	b.add("-XX:LargePageSizeInBytes=2M", "-XX:+UseLargePages")
}

func g1Refinement(b *builder) {
	b.when(b.env.before(24),
		"-XX:G1ConcRSHotCardLimit=16",
		"-XX:G1ConcRefinementServiceIntervalMillis=150",
	)
}

// biasedLocking was obsoleted in Java 18 and is rejected outright from then on.
func biasedLocking(b *builder) {
	b.when(b.env.before(18), "-XX:-UseBiasedLocking")
}

// x86Intrinsics are defined only in the x86 half of HotSpot; an aarch64 JVM
// reports every one of them as an unrecognized option and exits.
func x86Intrinsics(b *builder) {
	b.when(b.env.x86(),
		"-XX:+UseFPUForSpilling",
		"-XX:+UseFastStosb",
		"-XX:+UseNewLongLShift",
		"-XX:+UseVectorCmov",
		"-XX:+UseXMMForArrayCopy",
		"-XX:+UseXmmI2D",
		"-XX:+UseXmmI2F",
		"-XX:+UseXmmLoadAndClearUpper",
		"-XX:+UseXmmRegToRegMoveAll",
	)
}

func (p Preset) AllFlags(heapMB int) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range envVariants(heapMB) {
		for _, f := range p.Flags(e) {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

func envVariants(heapMB int) []Env {
	var out []Env
	for _, major := range []int{17, 21, 25} {
		for _, g := range []java.Graal{java.GraalNone, java.GraalOracle} {
			for _, goos := range []string{"linux", "windows"} {
				for _, arch := range []string{"amd64", "arm64"} {
					out = append(out, Env{HeapMB: heapMB, Major: major, Graal: g, GOOS: goos, GOARCH: arch})
				}
			}
		}
	}
	return out
}

func GetPreset(id string) Preset {
	if p, ok := presets[id]; ok {
		return p
	}
	return presets["none"]
}

var presetRank = map[string]int{
	"aikar":       0,
	"meowice":     1,
	"meowice-zgc": 2,
	"obydux":      3,
	"bruce":       4,
	"hilltty":     5,
	"etil":        6,
	"none":        7,
}

func Presets() []Preset {
	out := make([]Preset, 0, len(presets))
	for _, p := range presets {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := presetRank[out[i].ID], presetRank[out[j].ID]
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	return out
}
