# Adopt release-pilot: Implementation Plan

**Goal:** Replace raw goreleaser-action workflow with release-pilot composite action for AI-powered releases.

**Spec:** `docs/superpowers/specs/2026-04-01-release-pilot-adoption.md`

---

## Tasks

- [ ] **1. Add `.release-pilot.yaml`** — ecosystem: go, model: claude-sonnet-4-6, no signing
- [ ] **2. Rewrite `.github/workflows/release.yml`** — use `dakaneye/hookshot/release-pilot@<sha>`, pass `ANTHROPIC_API_KEY`, set `draft: true` for first run, add goreleaser setup step
- [ ] **3. Trim `.goreleaser.yaml`** — remove `release.header` and `release.footer` (release-pilot generates release body)
- [ ] **4. Verify locally** — `make lint && make test` pass
- [ ] **5. Manual: add `ANTHROPIC_API_KEY` secret** — repo settings > Secrets > Actions
- [ ] **6. Manual: test first release** — tag and push, verify draft release, then remove `draft: true`
