# Adopt release-pilot for Automated Releases

## Purpose

Replace the hand-rolled goreleaser release workflow with [release-pilot](https://github.com/dakaneye/release-pilot), which adds AI-powered version bumping and release note generation while keeping goreleaser for the actual build step.

## Decisions

| Decision | Answer |
|----------|--------|
| Composite action | `dakaneye/hookshot/release-pilot@<sha>` (pinned) |
| Trigger | Tag push (`v*`) — same as today |
| Runner | `macos-latest` — matches CI and darwin-only builds |
| Goreleaser config | Keep `.goreleaser.yaml` — release-pilot detects and delegates to it |
| Release notes | release-pilot generates via Claude — remove goreleaser `release.header`/`release.footer` to avoid duplication |
| Changelog grouping | Keep goreleaser `changelog` block — goreleaser changelog feeds into release-pilot's context |
| Config | `.release-pilot.yaml` with `ecosystem: go` |
| First release | Use `--draft` to verify end-to-end before going live |
| Signing | `sign: false` for now — can enable cosign later |
| Secret | `ANTHROPIC_API_KEY` repo secret (must be added manually) |
| Permissions | `contents: write` (create releases), `id-token: write` (future cosign OIDC) |

## What Changes

1. **New file**: `.release-pilot.yaml` — release-pilot config
2. **Rewritten**: `.github/workflows/release.yml` — uses hookshot composite action instead of raw goreleaser-action
3. **Modified**: `.goreleaser.yaml` — remove `release.header`/`release.footer` (release-pilot owns the release body)

## What Stays

- `.goreleaser.yaml` builds/archives/checksums — release-pilot delegates to it
- `changelog` block in goreleaser — used for goreleaser's own changelog generation which release-pilot can consume
- CI workflow — unchanged
- Tag-push trigger — unchanged

## Manual Steps (post-merge)

1. Add `ANTHROPIC_API_KEY` as a repo secret
2. Tag a release: `git tag v0.x.0 && git push origin v0.x.0`
3. Verify the draft release looks correct
4. Once validated, remove `draft: true` from workflow
