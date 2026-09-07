# AGENTS.md — ai-tiny-bot monorepo

Guidance for Cursor / Claude agents working in this repository.

## Layout

| Path | Role |
|------|------|
| [`tiny-bot-cloud-agent/`](tiny-bot-cloud-agent/) | Go cloud agent (WebSocket, ASR/LLM/TTS, persona/memory/skills) |
| [`tiny-bot-firmware/`](tiny-bot-firmware/) | ESP32 firmware (PlatformIO) |
| [`docs/`](docs/) | Hardware / build guides |
| [`.cursor/`](.cursor/) | Project hooks and rules |

Package-specific conventions:

- Cloud agent: [`tiny-bot-cloud-agent/CLAUDE.md`](tiny-bot-cloud-agent/CLAUDE.md)
- Firmware: [`tiny-bot-firmware/CLAUDE.md`](tiny-bot-firmware/CLAUDE.md)

## Ship checklist (every deliverable change)

When a task finishes with file changes, the project `stop` hook
(`.cursor/hooks/ship-on-stop.sh`) expects you to:

1. **Changelog** — append one bullet under `[Unreleased]` in the package you changed:
   - `tiny-bot-cloud-agent/CHANGELOG.md`
   - `tiny-bot-firmware/CHANGELOG.md`
   - Root / `.cursor/` workflow changes do **not** get a root changelog; update this `AGENTS.md` instead.
   - Hardware guides under `docs/` alone do **not** require an `AGENTS.md` bump.
2. **Agent docs** — update `AGENTS.md` and/or the package `CLAUDE.md` only when
   developer conventions, commands, layout, or durable workflow changed
   (e.g. `.cursor/hooks`, rules). Do **not** treat them as a second changelog for
   routine bugfixes or product/human docs.
3. **Commit** — English subject (why) + English body via HEREDOC. Never `--no-verify`.
   Stage only files for this task (`git add -A` is discouraged). Leave unrelated WIP unstaged.
4. **Push** — `git push -u origin HEAD`. This solo repo **allows** commit/push on `main`.
   Never force-push.

`afterFileEdit` records session edit paths under `.cursor/hooks/state/` (gitignored).
`ship-on-stop` only checks **this session's** edits, not pre-existing dirty files.

Hard gates live in `.cursor/hooks/git-pr-guard.sh` (before shell): no force-push,
no `--no-verify`, and a secrets scan of staged diffs / commit messages.

## Git remote (codepotter66)

This repo uses the `codepotter66` GitHub account. Remote should stay:

`git@github.com-codepotter66:codepotter66/ai-tiny-bot.git`

Use SSH key `~/.ssh/id_ed25519_codepotter66` (Host `github.com-codepotter66` in `~/.ssh/config`).
Do not push with other GitHub identities.

## Secrets — never commit or push

- `.env`, `.env.*` (`.env.example` is OK)
- `tiny-bot-firmware/include/config.h` (use `config.h.example`)
- `*.pem` / `*.key` / private key blocks
- Real passwords, API keys, tokens, real public IPs / host:port pairs

Documented placeholders (`YOUR_…`, `example.com`, private RFC1918 ranges, protocol default `:5678` in docs) are fine.

## Git safety (also enforced by hooks)

- No `git push --force` / `-f` to shared remotes
- Commit/push on `main` is allowed (solo repo)
- No `git reset --hard` or `git clean -f` without explicit user request
