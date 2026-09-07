#!/usr/bin/env bash
# stop: if the working tree has shippable work, ask the agent to update docs,
# commit in English, and push the current feature branch.
set -euo pipefail

input=$(cat)

export SHIP_ON_STOP_INPUT="$input"
python3 - <<'PY'
import json
import os
import re
import subprocess
import sys


def emit(followup_message=None):
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

# Resolve repo root (script may run from anywhere).
repo = run(["git", "rev-parse", "--show-toplevel"])
if repo is None or repo.returncode != 0:
    emit()
root = repo.stdout.strip()
os.chdir(root)

branch_p = run(["git", "rev-parse", "--abbrev-ref", "HEAD"])
branch = (branch_p.stdout.strip() if branch_p and branch_p.returncode == 0 else "")

# Dirty / untracked paths (name-status for changed + untracked).
status_p = run(["git", "status", "--porcelain", "-u"])
porcelain = status_p.stdout if status_p and status_p.returncode == 0 else ""

changed_paths = []
for line in porcelain.splitlines():
    if not line.strip():
        continue
    # formats: XY PATH, XY ORIG -> PATH, ?? PATH
    rest = line[3:] if len(line) > 3 else line
    if " -> " in rest:
        rest = rest.split(" -> ", 1)[1]
    path = rest.strip()
    if path:
        changed_paths.append(path)

# Ahead of upstream?
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

if not changed_paths and ahead == 0:
    emit()

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
        ) or base.lower() in ("changelog.md", "claude.md", "agents.md") and path.startswith(
            package + "/"
        )
    return path in ("AGENTS.md", "CHANGELOG.md", "CLAUDE.md") or base.lower() in (
        "agents.md",
        "changelog.md",
        "claude.md",
    )


CONV_PATTERNS = (
    re.compile(r"(^|/)\.cursor/"),
    re.compile(r"(^|/)README(\.|$)"),
    re.compile(r"(^|/)AGENTS\.md$", re.I),
    re.compile(r"(^|/)CLAUDE\.md$", re.I),
    re.compile(r"hooks\.json$"),
    re.compile(r"\.cursor/hooks/"),
    re.compile(r"\.cursor/rules/"),
    re.compile(r"ws-protocol\.schema\.json$"),
    re.compile(r"http-api\.openapi\.ya?ml$"),
    re.compile(r"docs/human-docs/"),
    re.compile(r"docs/firmware-integration/"),
    re.compile(r"Makefile$"),
    re.compile(r"docker-compose\.ya?ml$"),
    re.compile(r"Dockerfile"),
    re.compile(r"\.env\.example$"),
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

# --- cloud-agent ---
cloud_code = [p for p in cloud_paths if not is_self_doc(p, CLOUD)]
if cloud_code:
    changelog = f"{CLOUD}/CHANGELOG.md"
    if changelog not in changed_paths:
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
        if not any(d in changed_paths for d in agent_docs):
            issues.append(
                f"Convention/docs changes under `{CLOUD}/` require updating "
                f"`{CLOUD}/CLAUDE.md` and/or root `AGENTS.md`."
            )
            actions.append(
                f"Update `{CLOUD}/CLAUDE.md` or root `AGENTS.md` only if "
                f"developer conventions, commands, or layout changed "
                f"(do not duplicate the changelog)."
            )

# --- firmware ---
fw_code = [p for p in fw_paths if not is_self_doc(p, FW)]
if fw_code:
    changelog = f"{FW}/CHANGELOG.md"
    if changelog not in changed_paths:
        issues.append(
            f"Changes under `{FW}/` require an entry in `{changelog}` "
            f"under `[Unreleased]`."
        )
        actions.append(
            f"Append one concise bullet to `{changelog}` `[Unreleased]`."
        )
    if any(is_convention_file(p) for p in fw_code):
        agent_docs = (f"{FW}/CLAUDE.md", "AGENTS.md")
        if not any(d in changed_paths for d in agent_docs):
            issues.append(
                f"Convention/docs changes under `{FW}/` require updating "
                f"`{FW}/CLAUDE.md` and/or root `AGENTS.md`."
            )
            actions.append(
                f"Update `{FW}/CLAUDE.md` or root `AGENTS.md` if conventions "
                f"changed (not a second changelog)."
            )

# --- root / docs / .cursor ---
root_code = [p for p in root_paths if not is_self_doc(p, "")]
if root_code:
    if "AGENTS.md" not in changed_paths:
        # Always require AGENTS.md for non-doc-only root changes that are
        # convention-related OR any substantive root/docs/.cursor change.
        needs_agents = any(
            is_convention_file(p) or p.startswith("docs/") or p.startswith(".cursor/")
            for p in root_code
        ) or bool(root_code)
        if needs_agents:
            issues.append(
                "Root / `docs/` / `.cursor/` changes require updating root `AGENTS.md`."
            )
            actions.append(
                "Update root `AGENTS.md` with the durable repo-level guidance "
                "(no root CHANGELOG)."
            )

# Unpushed commits
if ahead > 0 and branch not in ("main", "master"):
    issues.append(
        f"Branch `{branch}` is {ahead} commit(s) ahead of upstream and not pushed."
    )
    actions.append("Push with `git push -u origin HEAD` (never to main/master).")

# Dirty tree needs commit (when not blocked on main)
if changed_paths and branch not in ("main", "master"):
    issues.append("Working tree has uncommitted changes.")
    actions.append(
        "Stage only files for this task (never `git add -A`). "
        "Write an English commit subject (why) + body via HEREDOC. "
        "Do not use `--no-verify`. Then `git push -u origin HEAD`."
    )

# Sensitive-looking staged/unstaged paths (soft reminder; hard gate is in git-pr-guard)
SENSITIVE_NAME = re.compile(
    r"(^|/)\.env($|\.)|(^|/)config\.h$|\.pem$|\.key$|(^|/)id_rsa|(^|/)credentials",
    re.I,
)
bad_names = [p for p in changed_paths if SENSITIVE_NAME.search(p) and not p.endswith(".env.example")]
if bad_names:
    issues.append(
        "Sensitive-looking paths are dirty: " + ", ".join(f"`{p}`" for p in bad_names[:8])
    )
    actions.append(
        "Do not stage or commit those files. Keep secrets in gitignored local files only."
    )

if not issues:
    # Clean enough: docs already updated, or only ahead=0 with nothing else.
    # If dirty but somehow no issues (shouldn't happen), still ask to ship.
    if changed_paths and branch not in ("main", "master"):
        issues.append("Working tree has uncommitted changes.")
        actions.append(
            "Ensure CHANGELOG / AGENTS.md / CLAUDE.md are updated as required, "
            "commit in English via HEREDOC, then push the feature branch."
        )
    elif ahead > 0:
        pass  # already covered
    else:
        emit()

# Deduplicate while preserving order
seen = set()
uniq_actions = []
for a in actions:
    if a not in seen:
        seen.add(a)
        uniq_actions.append(a)

msg_lines = [
    "Ship-on-stop check failed. Finish shipping this change before ending:",
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
        "- Scope the commit to this task only.",
    ]
)

emit("\n".join(msg_lines))
PY
