#!/usr/bin/env bash
# stop: if THIS session's edits need shipping, ask the agent to update docs,
# commit in English, and push the current feature branch.
# Unrelated pre-existing WIP is ignored (scoped via afterFileEdit tracker).
set -euo pipefail

input=$(cat)

export SHIP_ON_STOP_INPUT="$input"
python3 - <<'PY'
import json
import os
import re
import subprocess
import sys


def emit(followup_message=None, clear_session=False, state_file=None):
    if clear_session and state_file and os.path.isfile(state_file):
        try:
            os.remove(state_file)
        except OSError:
            pass
    out = {}
    if followup_message:
        out["followup_message"] = followup_message
    print(json.dumps(out, ensure_ascii=False))
    sys.exit(0)


def run(cmd, check=False):
    try:
        return subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=check,
        )
    except Exception:
        return None


try:
    data = json.loads(os.environ.get("SHIP_ON_STOP_INPUT") or "{}")
except Exception:
    emit()

status = (data.get("status") or "").strip()
if status == "aborted":
    emit()

repo = run(["git", "rev-parse", "--show-toplevel"])
if repo is None or repo.returncode != 0:
    emit()
root = repo.stdout.strip()
os.chdir(root)

state_file = os.path.join(root, ".cursor", "hooks", "state", "edited-paths")

branch_p = run(["git", "rev-parse", "--abbrev-ref", "HEAD"])
branch = (branch_p.stdout.strip() if branch_p and branch_p.returncode == 0 else "")

# All dirty paths (for intersect + sensitive checks).
status_p = run(["git", "status", "--porcelain", "-u"])
porcelain = status_p.stdout if status_p and status_p.returncode == 0 else ""

dirty_paths = []
for line in porcelain.splitlines():
    if not line.strip():
        continue
    rest = line[3:] if len(line) > 3 else line
    if " -> " in rest:
        rest = rest.split(" -> ", 1)[1]
    path = rest.strip()
    if path:
        dirty_paths.append(path)
dirty_set = set(dirty_paths)

# Session-scoped edits from afterFileEdit. If none, do not nag about unrelated WIP.
session_paths = []
if os.path.isfile(state_file):
    with open(state_file, encoding="utf-8") as f:
        session_paths = [ln.strip() for ln in f if ln.strip()]

# Only paths this agent session touched that are still dirty (or new).
changed_paths = [p for p in session_paths if p in dirty_set]
# Also include session paths that are clean but were edited then committed mid-turn:
# if session non-empty but intersection empty and ahead==0 → clear and done.

ahead = 0
upstream_p = run(["git", "rev-parse", "--abbrev-ref", "@{upstream}"])
has_upstream = upstream_p is not None and upstream_p.returncode == 0
if has_upstream:
    counts = run(["git", "rev-list", "--left-right", "--count", "HEAD...@{upstream}"])
    if counts and counts.returncode == 0:
        parts = counts.stdout.strip().split()
        if len(parts) >= 1:
            try:
                ahead = int(parts[0])
            except ValueError:
                ahead = 0

# No session edits → ignore leftover WIP; only remind if commits are unpushed.
if not session_paths:
    if ahead > 0 and branch not in ("main", "master"):
        emit(
            "Ship-on-stop: branch is ahead of upstream. "
            "Push with `git push -u origin HEAD` (never main/master / force-push)."
        )
    emit(clear_session=True, state_file=state_file)

# Session edits already committed (no longer dirty) and pushed → done.
if not changed_paths and ahead == 0:
    emit(clear_session=True, state_file=state_file)

# Session edits committed but not pushed.
if not changed_paths and ahead > 0 and branch not in ("main", "master"):
    emit(
        "Ship-on-stop: this session's commits are not pushed. "
        "Run `git push -u origin HEAD` (never main/master / force-push)."
    )

issues = []
actions = []

if branch in ("main", "master"):
    issues.append(
        f"Current branch is `{branch}`. Do not commit or push on main/master."
    )
    actions.append(
        "Create a feature branch (`git switch -c <name>`), move the work there, "
        "then commit and push that branch."
    )

CLOUD = "tiny-bot-cloud-agent"
FW = "tiny-bot-firmware"


def under(prefix, paths):
    return [p for p in paths if p == prefix or p.startswith(prefix + "/")]


def is_self_doc(path, package):
    base = path.rsplit("/", 1)[-1]
    if package:
        return path in (
            f"{package}/CHANGELOG.md",
            f"{package}/CLAUDE.md",
            f"{package}/AGENTS.md",
        ) or (
            base.lower() in ("changelog.md", "claude.md", "agents.md")
            and path.startswith(package + "/")
        )
    return path in ("AGENTS.md", "CHANGELOG.md", "CLAUDE.md") or base.lower() in (
        "agents.md",
        "changelog.md",
        "claude.md",
    )


# Only developer-workflow / agent-convention files — not product docs.
CONV_PATTERNS = (
    re.compile(r"(^|/)\.cursor/"),
    re.compile(r"(^|/)AGENTS\.md$", re.I),
    re.compile(r"(^|/)CLAUDE\.md$", re.I),
    re.compile(r"hooks\.json$"),
    re.compile(r"\.cursor/hooks/"),
    re.compile(r"\.cursor/rules/"),
)


def is_convention_file(path):
    return any(p.search(path) for p in CONV_PATTERNS)


cloud_paths = under(CLOUD, changed_paths)
fw_paths = under(FW, changed_paths)
root_paths = [
    p
    for p in changed_paths
    if not p.startswith(CLOUD + "/")
    and p != CLOUD
    and not p.startswith(FW + "/")
    and p != FW
]

cloud_code = [p for p in cloud_paths if not is_self_doc(p, CLOUD)]
if cloud_code:
    changelog = f"{CLOUD}/CHANGELOG.md"
    # Satisfied if changelog is in this session's dirty set OR already dirty from same work.
    if changelog not in dirty_set:
        issues.append(
            f"Changes under `{CLOUD}/` require an entry in `{changelog}` "
            f"under `[Unreleased]`."
        )
        actions.append(
            f"Append one concise bullet to `{changelog}` `[Unreleased]` "
            f"(Added/Changed/Fixed as appropriate)."
        )
    if any(is_convention_file(p) for p in cloud_code):
        agent_docs = (f"{CLOUD}/CLAUDE.md", "AGENTS.md")
        if not any(d in dirty_set for d in agent_docs):
            issues.append(
                f"Convention changes under `{CLOUD}/` require updating "
                f"`{CLOUD}/CLAUDE.md` and/or root `AGENTS.md`."
            )
            actions.append(
                f"Update `{CLOUD}/CLAUDE.md` or root `AGENTS.md` only if "
                f"developer conventions, commands, or layout changed "
                f"(do not duplicate the changelog)."
            )

fw_code = [p for p in fw_paths if not is_self_doc(p, FW)]
if fw_code:
    changelog = f"{FW}/CHANGELOG.md"
    if changelog not in dirty_set:
        issues.append(
            f"Changes under `{FW}/` require an entry in `{changelog}` "
            f"under `[Unreleased]`."
        )
        actions.append(
            f"Append one concise bullet to `{changelog}` `[Unreleased]`."
        )
    if any(is_convention_file(p) for p in fw_code):
        agent_docs = (f"{FW}/CLAUDE.md", "AGENTS.md")
        if not any(d in dirty_set for d in agent_docs):
            issues.append(
                f"Convention changes under `{FW}/` require updating "
                f"`{FW}/CLAUDE.md` and/or root `AGENTS.md`."
            )
            actions.append(
                f"Update `{FW}/CLAUDE.md` or root `AGENTS.md` if conventions "
                f"changed (not a second changelog)."
            )

root_code = [p for p in root_paths if not is_self_doc(p, "")]
if root_code:
    # Hardware docs alone do not require AGENTS.md; .cursor / workflow does.
    needs_agents = any(
        is_convention_file(p) or p.startswith(".cursor/") for p in root_code
    )
    if needs_agents and "AGENTS.md" not in dirty_set:
        issues.append(
            "`.cursor/` / agent-workflow changes require updating root `AGENTS.md`."
        )
        actions.append(
            "Update root `AGENTS.md` with the durable repo-level guidance "
            "(no root CHANGELOG)."
        )

if ahead > 0 and branch not in ("main", "master"):
    issues.append(
        f"Branch `{branch}` is {ahead} commit(s) ahead of upstream and not pushed."
    )
    actions.append("Push with `git push -u origin HEAD` (never to main/master).")

if changed_paths and branch not in ("main", "master"):
    issues.append(
        "This session has uncommitted edits (scoped; unrelated WIP may remain)."
    )
    actions.append(
        "Stage only files for this task (never `git add -A`). "
        "Write an English commit subject (why) + body via HEREDOC. "
        "Do not use `--no-verify`. Then `git push -u origin HEAD`."
    )

SENSITIVE_NAME = re.compile(
    r"(^|/)\.env($|\.)|(^|/)config\.h$|\.pem$|\.key$|(^|/)id_rsa|(^|/)credentials",
    re.I,
)
bad_names = [
    p
    for p in changed_paths
    if SENSITIVE_NAME.search(p) and not p.endswith(".env.example")
]
if bad_names:
    issues.append(
        "Sensitive-looking paths were edited: "
        + ", ".join(f"`{p}`" for p in bad_names[:8])
    )
    actions.append(
        "Do not stage or commit those files. Keep secrets in gitignored local files only."
    )

if not issues:
    emit(clear_session=True, state_file=state_file)

seen = set()
uniq_actions = []
for a in actions:
    if a not in seen:
        seen.add(a)
        uniq_actions.append(a)

scope_list = ", ".join(f"`{p}`" for p in changed_paths[:12])
if len(changed_paths) > 12:
    scope_list += f", … (+{len(changed_paths) - 12} more)"

msg_lines = [
    "Ship-on-stop check failed. Finish shipping THIS session's edits before ending:",
    "",
    f"Session edit scope: {scope_list}",
    "",
    "Issues:",
]
for i in issues:
    msg_lines.append(f"- {i}")
msg_lines.append("")
msg_lines.append("Required actions:")
for a in uniq_actions:
    msg_lines.append(f"- {a}")
msg_lines.extend(
    [
        "",
        "Rules:",
        "- Commit subject and body must be English.",
        "- Never commit secrets: `.env`, passwords, API keys, real public IPs/ports, `config.h`, `*.pem`/`*.key`.",
        "- Never push main/master or force-push.",
        "- Do not treat CLAUDE.md/AGENTS.md as a second changelog; only update them for conventions/commands/layout.",
        "- Scope the commit to this task only; leave unrelated WIP unstaged.",
    ]
)

emit("\n".join(msg_lines))
PY
