<!--
^ Summarise what this changes and why, above this comment. ^

Title this pull request as a Conventional Commit — `feat(scope): …`, `fix: …`.
Release notes are generated from commit titles, so a title that does not parse
lands under "💬 Other" instead of its section.

If it closes an issue, write "Closes #123" so GitHub links the two.
If it changes behaviour a user can see, say what the old behaviour was.
-->

## Things done

<!-- Check what applies. These are not hard requirements — they tell a reviewer
     what you have already covered and where to look. Delete a group that has
     nothing to do with this change rather than leaving it unchecked. -->

- Tested, as applicable:
  - [ ] Manually walked through the behaviour this changes.
  - [ ] Covered by a new or updated test under `test/`.
  - [ ] Nothing to test — documentation, formatting or build-only change.
- [ ] Changes to a server config file go through the one writer, not a bespoke
      `os.WriteFile` — an ad-hoc write is how a `server.properties` loses the
      comments and ordering around it.
- [ ] Nix: where this changes the build inputs, the binary's arguments or a
      runtime dependency, `flake.nix` and `nix/` were updated with it. `nix build
      .#irori` runs as a required check and will find the omission anyway.
- [ ] The installers in `installer/` still describe the assets a release actually
      produces, where this changes their names or platforms.
- [ ] This pull request has one subject. A formatting sweep bundled with a fix is
      harder to review and harder to revert.
- [ ] Fits [CONTRIBUTING.md].

<!--
Want a build of this branch to review? Add the "📦 upload artifacts" label and the
PR check attaches it to its run — the Artifacts box at the bottom of the run
summary. The build runs either way; only the upload depends on the label.

Found a security issue? Do not open a pull request in the open — see [SECURITY.md].
-->

[CONTRIBUTING.md]: https://github.com/BX-Team/.github/blob/master/CONTRIBUTING.md
[SECURITY.md]: https://github.com/BX-Team/.github/blob/master/SECURITY.md
