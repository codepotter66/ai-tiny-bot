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
   - Root / `docs/` / `.cursor/` changes do **not** get a root changelog; update this `AGENTS.md` instead.
2. **Agent docs** — update `AGENTS.md` and/or the package `CLAUDE.md` only when
   developer conventions, commands, layout, or durable workflow changed.
   Do **not** treat them as a second changelog for routine bugfixes.
3. **Commit** — English subject (why) + English body via HEREDOC. Never `--no-verify`.
   Stage only files for this task (`git add -A` is discouraged).
4. **Push** — `git push -u origin HEAD` on a **feature branch**. Never push `main`/`master`, never force-push.

Hard gates live in `.cursor/hooks/git-pr-guard.sh` (before shell): no force-push,
no commit/push on main, no `--no-verify`, and a secrets scan of staged diffs / commit messages.

## Secrets — never commit or push

- `.env`, `.env.*` (`.env.example` is OK)
- `tiny-bot-firmware/include/config.h` (use `config.h.example`)
- `*.pem` / `*.key` / private key blocks
- Real passwords, API keys, tokens, real public IPs / host:port pairs

Documented placeholders (`YOUR_…`, `example.com`, private RFC1918 ranges, protocol default `:5678` in docs) are fine.

## Git safety (also enforced by hooks)

- No `git push --force` / `-f` to shared remotes
- No commits or pushes directly to `main` / `master`
- No `git reset --hard` or `git clean -f` without explicit user request
