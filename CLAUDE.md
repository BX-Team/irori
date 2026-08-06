# irori

A terminal UI for running a Minecraft server. Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea), one binary: the CLI, the TUI and the supervising daemon all live in `cmd/irori`.

## Architecture

irori runs **in the server directory**. `.irori.json` next to `server.properties` is the whole configuration; the CLI walks up from the working directory to find it (`config.Find`).

The server process is owned by a **detached irori daemon**, not by the TUI and not by tmux. The TUI is a client: it connects to a unix socket, streams console output and state, and sends commands. Closing the TUI leaves the server running.

| Package             | Responsibility                                                                                       |
| ------------------- | ---------------------------------------------------------------------------------------------------- |
| `internal/cli`      | cobra commands: TUI (default), `daemon`, `start`/`stop`/`restart`/`kill`/`status`/`cmd`/`logs`, `apply`, `config`, `nix`, `defaults`. |
| `internal/daemon`   | The supervisor: spawns java, scans console output, tracks state/players/stats, restarts, serves the socket. |
| `internal/ipc`      | Wire protocol between daemon and clients (newline-delimited JSON frames over a unix socket).           |
| `internal/config`   | `.irori.json`, `.irori.lock.json`, paths (socket, state dir, user config/cache), sealed mode.          |
| `internal/host`     | `host.Backend` — every filesystem touch goes through it, so an SSH backend can be added without rewriting callers. |
| `internal/mcjars`   | mcjars.app client: cores, versions, builds, install recipes, the config files a build ships.           |
| `internal/modrinth` | Modrinth search and downloads.                                                                        |
| `internal/apply`    | `irori apply` — installs the declared core and plugins, prunes what is no longer declared.             |
| `internal/install`  | The one path a jar reaches the directory by; always records what it installed in the lock.             |
| `internal/confdiff` | Diffs the configs the core ships against the ones on disk, for `irori config import`.                 |
| `internal/lock`     | `.irori.lock.json`: every URL and checksum irori installed.                                            |
| `internal/nixgen`   | Renders the lock as fixed-output derivations for the NixOS module.                                     |
| `internal/overrides`| Writes declared keys back into properties/yml/toml **preserving comments**. The single writer for config files. |
| `internal/confs`    | Discovers and parses the core's config files into flat dotted-key entries.                            |
| `internal/props`    | `server.properties` catalog: types, ranges, enums.                                                    |
| `internal/java`     | JDK discovery (JAVA_HOME, PATH, system globs, `/nix/store`), version floors, GraalVM edition, caching. |
| `internal/launch`   | Builds the java command line: heap, GC flag presets, jar, extra args.                                 |
| `internal/importer` | Adopts an existing server directory by reading its start script.                                      |
| `internal/ui`       | The TUI: `app.go` (root model, tab routing), `screens/` (one per tab), `components/`, `theme/`, `wizard/`, `link/` (daemon connection). |

Tabs, in order: **Console** (dashboard), **Files**, **Plugins/Mods**, **Configs**, **Settings**.

### Decisions that are settled

- **The daemon owns the process, not tmux.** Any "just attach to a screen session" idea has already been rejected.
- **Configs are edited two ways**: `$EDITOR` through `tea.ExecProcess`, and declaratively as key overrides in `.irori.json` applied by `irori apply`. Both go through `internal/overrides` so comments survive.
- **A config key's description is the comment its own file carries.** There is deliberately no hand-written catalog for third-party cores — a fork's own comments are the documentation. `server.properties` is the exception and keeps `props.Catalog`.
- **Java detection has two floors**: `cfg.Java.Major` from mcjars, otherwise `java.RequiredFor(mcVersion)`. With no floor at all, prefer the **newest** JDK — preferring the lowest is what made a NixOS box try to start a modern server on a stray Java 8.
- **A flag preset is a function of the JVM, not a constant.** `launch.Preset.Flags` takes a `launch.Env` (heap, Java major, GraalVM edition, GOOS/GOARCH) because a JVM refuses to start on an option it does not know rather than warning: `-XX:G1ConcRSHotCardLimit` and `ShenandoahGCMode=iu` are gone in Java 24, `UseBiasedLocking` in 18, `UseCompactObjectHeaders` arrives in 24, the `-XX:+UseXmm*` intrinsics do not exist on aarch64, and the `-Djdk.graal.*` options only exist on Oracle's GraalVM (spelled `-Dgraal.*` before its JDK 23 release). The unlock options must also come *first* in the list, not merely be present. Every boundary was checked against a real JDK 17, 21 and 25; `TestPresetsStartTheLocalJVM` re-checks them on whatever JDK is on PATH. Defaults: **aikar**, then **meowice**.
- **The lock is written by whoever installs, not by `irori apply`.** Every install path goes through `internal/install`, which saves `.irori.lock.json` before it returns; the caller saves `.irori.json`. `irori apply` converges a directory it did not build, it is not a step you must remember after one.
- **Declared config keys are an overlay, never a replacement.** The core generates its own config, then irori writes the declared keys over it through `internal/overrides`. On NixOS the module does this in `preStart` with the values `irori nix` rendered into the artifacts file. Plugin configs are deliberately out of scope: no pristine copy of one exists to diff against.
- **On NixOS the daemon runs as a systemd unit** (`nix/module.nix`, `services.irori.servers.<name>`), not by self-detaching. Jars and plugins come from the store: `irori nix` renders the lock as `fetchurl` FODs and the unit sets `IRORI_SEALED=1`, which makes irori refuse to download anything.

## Commands

```bash
go build ./...          # build
go run ./cmd/irori      # run the TUI in the current server directory
go test ./...           # test
go vet ./...
gofmt -l $(git ls-files '*.go')
golangci-lint run       # config: .golangci.yaml
nix build .#irori       # the flake package
```

CI runs gofmt, `go vet`, `go test`, golangci-lint and a cross-compile matrix — all of it must pass before a commit. The daemon is the one package with per-OS files (`proc_unix.go` / `proc_windows.go`), so check `GOOS=windows go build ./...` after touching it.

## Code Guidelines

### Comments
- NO file-header banners and NO divider comments (`// --- helpers ---`). Group code with functions, not comment art.
- Add an inline comment only where the code is genuinely non-obvious — a real footgun, a wire-format quirk, a reason a thing is done backwards. Then keep it to a line or two.
- Don't narrate the obvious. If a comment restates the next line, delete it.
- Package doc comments are fine and should say *why* the package exists, in one or two sentences.

### Style
- gofmt is the source of truth — never hand-format against it.
- Match the surrounding code. Screens are Bubble Tea models with `Update`/`View`; follow the idiom already in the file you're editing.
- Colors come from `internal/ui/theme` (default Catppuccin Mocha; add a palette by appending one entry to `all`). Never hardcode a color outside `theme/`.
- **Themes are foregrounds only.** irori never paints a page background — it draws over whatever the terminal already is. `Mantle` backs modals, `Surface` the bars, `Crust` is the text on an accent chip; nothing covers the screen. Two consequences, both already paid for: **no light palettes** (Latte was unreadable on a dark terminal), and **no near-identical siblings** — Catppuccin's four flavours differ mainly in the background ramp, so on screen only Mocha's foregrounds survive and it ships alone as `catppuccin`.
- All filesystem access goes through `host.Backend`, not `os` directly — that indirection is what keeps a remote backend possible.
- All config writes go through `internal/overrides`. Writing a properties or yml file by hand loses the comments.

### UI language
- Every user-facing string ships in **English**, whatever language the conversation is in. Labels, hints, toasts, errors, help text — all English.

### TUI gotchas
- `lipgloss.Width()` counts padding but **not** borders. A panel that fits by that measurement still overflows by 2 columns once it is bordered — subtract the border yourself when sizing.
- Key events go to the **focused tab only**; state updates broadcast to every screen. Never route a key by scanning all screens.
- Mouse zones come from bubblezone; a clickable region must be wrapped in a zone with a stable id or the hit test silently does nothing.

### Testing
- **All tests live in `test/`** (package `irori_test`), black-box, importing the internal packages. Don't scatter `_test.go` files next to the code.
- One test per real trap, not one per function. The panel layout math, the comment-preserving writers, the Java version floors and the Nix expression shape earn tests; a getter does not.
- Prefer a test that would have caught a bug we actually hit, and say in a comment what breaks if it fails.

## Bash Guidelines
- Don't pipe output through `head`/`tail`/`less` to truncate — use tool-native flags (`git log -n 10`, `go test ./internal/confs`). Read the full output.
- Don't create scratch files (scripts, notes) unless asked.
- When given failures, just fix them — don't argue about who introduced them.
