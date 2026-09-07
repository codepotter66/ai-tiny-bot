#!/usr/bin/env bash
# beforeShellExecution: deny dangerous git ops; scan staged secrets on commit/push;
# remind on commit / gh pr create.
set -euo pipefail

input=$(cat)

export GIT_PR_GUARD_INPUT="$input"
python3 - <<'PY'
import json
import os
import re
import subprocess
import sys


def emit(permission, agent_message=None, user_message=None):
    out = {"permission": permission}
    if agent_message:
        out["agent_message"] = agent_message
    if user_message:
        out["user_message"] = user_message
    print(json.dumps(out, ensure_ascii=False))
    sys.exit(0)


try:
    data = json.loads(os.environ.get("GIT_PR_GUARD_INPUT") or "")
except Exception:
    emit("allow")

raw = data.get("command") or ""
if not raw.strip():
    emit("allow")


def strip_quotes_and_heredocs(s: str) -> str:
    """Remove quoted / heredoc payloads so message text cannot trigger deny rules."""
    out = []
    i = 0
    n = len(s)
    while i < n:
        if s.startswith("<<", i):
            m = re.match(r"<<(-?)\s*(['\"]?)(\w+)\2", s[i:])
            if m:
                out.append(" ")
                i += m.end()
                delim = m.group(3)
                while i < n:
                    nl = s.find("\n", i)
                    if nl < 0:
                        i = n
                        break
                    line = s[i:nl]
                    i = nl + 1
                    if line.strip() == delim:
                        break
                continue
        ch = s[i]
        if ch in ("'", '"'):
            quote = ch
            out.append(" ")
            i += 1
            while i < n:
                if s[i] == "\\" and quote == '"':
                    i += 2
                    continue
                if s[i] == quote:
                    i += 1
                    break
                i += 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def extract_commit_message(s: str) -> str:
    """Best-effort extract of -m / heredoc commit message bodies for secret scan."""
    parts = []
    # HEREDOC: -m "$(cat <<'EOF' ... EOF)" or <<EOF
    for m in re.finditer(
        r"<<(-?)\s*(['\"]?)(\w+)\2\n(.*?)(?:\n\3\b)",
        s,
        re.DOTALL,
    ):
        parts.append(m.group(4))
    # Simple -m "..." / -m '...'
    for m in re.finditer(r"(?:^|\s)-m\s+\"([^\"]*)\"", s):
        parts.append(m.group(1))
    for m in re.finditer(r"(?:^|\s)-m\s+'([^']*)'", s):
        parts.append(m.group(1))
    return "\n".join(parts)


def split_segments(s: str):
    parts = re.split(r"(?:&&|\|\||;|\|(?!\|))", s)
    return [p.strip() for p in parts if p and p.strip()]


def git_argv(segment: str):
    m = re.match(r"^(?:[A-Za-z_][\w:]*=\S*\s+)*git\b(.*)$", segment, re.DOTALL)
    if not m:
        return None
    rest = m.group(1).strip()
    return rest.split() if rest else []


def gh_argv(segment: str):
    m = re.match(r"^(?:[A-Za-z_][\w:]*=\S*\s+)*gh\b(.*)$", segment, re.DOTALL)
    if not m:
        return None
    rest = m.group(1).strip()
    return rest.split() if rest else []


def git_subcommand(argv):
    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("-C", "-c", "--git-dir", "--work-tree", "--namespace"):
            i += 2
            continue
        if a.startswith("-"):
            i += 1
            continue
        return a, argv[i + 1 :]
    return None, []


scrubbed = strip_quotes_and_heredocs(raw)
segments = split_segments(scrubbed)

git_segments = []
gh_segments = []
for seg in segments:
    if re.match(r"^(?:[A-Za-z_][\w:]*=\S*\s+)*git\b", seg):
        git_segments.append(seg)
    elif re.match(r"^(?:[A-Za-z_][\w:]*=\S*\s+)*gh\b", seg):
        gh_segments.append(seg)

if not git_segments and not gh_segments:
    emit("allow")


CONV_SUBJECT_RE = re.compile(
    r"^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)"
    r"(\([a-zA-Z0-9/_.,-]+\))?!?: .+"
)


def has_token(args, tokens):
    return any(a in tokens for a in args)


def has_short_f(args):
    for a in args:
        if a == "-f":
            return True
        if re.fullmatch(r"-[a-zA-Z]*f[a-zA-Z]*", a) and not a.startswith("--"):
            return True
    return False


# --- secret / sensitive path scanning ---

FORBIDDEN_PATH_RE = re.compile(
    r"(^|/)\.env($|\.(?!example$))|"
    r"(^|/)tiny-bot-firmware/include/config\.h$|"
    r"\.(pem|key)$|"
    r"(^|/)id_rsa|"
    r"BEGIN\s+(RSA\s+)?PRIVATE\s+KEY",
    re.I,
)

# Assignment-like secrets (new/added lines). Placeholders often use YOUR_ / CHANGE_ME / xxx.
SECRET_ASSIGN_RE = re.compile(
    r"(?i)(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?key|private[_-]?key|"
    r"auth[_-]?token|TB_[A-Z0-9_]*(?:API_KEY|SECRET|TOKEN|PASSWORD))\s*"
    r"[=:]\s*['\"]?(?!YOUR_|CHANGE_ME|CHANGEME|REPLACE_|<.*>|\$\{?[A-Z_]+\}?|xxx+|TODO|"
    r"example|placeholder|null|none|empty)([^\s'\"]{8,})"
)

IPV4_RE = re.compile(
    r"(?<!\d)(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)(?!\d)"
)

# Documented / safe examples that must not trip the scanner.
SAFE_IP_PREFIXES = (
    "127.",
    "10.",
    "192.168.",
    "0.0.0.0",
    "255.255.255.255",
)
# 172.16.0.0 – 172.31.255.255
PRIVATE_172 = re.compile(r"^172\.(1[6-9]|2\d|3[0-1])\.")

# Common example hosts in this repo's docs
SAFE_EXAMPLE_HOSTS = re.compile(
    r"(?i)\b(?:localhost|example\.com|example\.org|tinypal|0\.0\.0\.0)\b"
)

# Protocol-default ports that appear in public docs (not a secret by themselves).
PUBLIC_DOC_PORTS = {"5678", "80", "443", "8080", "8443"}

HOST_PORT_RE = re.compile(
    r"(?<!\d)((?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d))"
    r":(\d{2,5})\b"
)


def is_private_or_safe_ip(ip: str) -> bool:
    if ip.startswith(SAFE_IP_PREFIXES):
        return True
    if PRIVATE_172.match(ip):
        return True
    # Documentation / TEST-NET (RFC 5737)
    if ip.startswith(("192.0.2.", "198.51.100.", "203.0.113.")):
        return True
    return False


def is_placeholder_line(line: str) -> bool:
    low = line.lower()
    markers = (
        "example",
        "placeholder",
        "your_",
        "change_me",
        "changeme",
        "replace_",
        "todo",
        "xxx",
        "<host>",
        "<ip>",
        "sample",
    )
    return any(m in low for m in markers)


def scan_text_for_secrets(text: str, source: str) -> list:
    hits = []
    for i, line in enumerate(text.splitlines(), 1):
        # Only care about added lines in diffs
        if line.startswith("+++") or line.startswith("---"):
            continue
        content = line[1:] if line.startswith("+") else line
        if line.startswith("+") or source == "message":
            pass
        elif source == "diff" and not line.startswith("+"):
            continue

        if is_placeholder_line(content):
            continue

        if FORBIDDEN_PATH_RE.search(content) and "PRIVATE KEY" in content.upper():
            hits.append(f"{source}:{i}: private key material")
            continue

        if SECRET_ASSIGN_RE.search(content):
            hits.append(f"{source}:{i}: secret-like assignment")
            continue

        for m in HOST_PORT_RE.finditer(content):
            ip, port = m.group(1), m.group(2)
            if is_private_or_safe_ip(ip):
                continue
            if port in PUBLIC_DOC_PORTS and SAFE_EXAMPLE_HOSTS.search(content):
                continue
            # Public IP + port is suspicious outside pure docs placeholders
            hits.append(f"{source}:{i}: public IP:port {ip}:{port}")

        for m in IPV4_RE.finditer(content):
            ip = m.group(0)
            if is_private_or_safe_ip(ip):
                continue
            # Bare public IP in non-placeholder line
            hits.append(f"{source}:{i}: public IP {ip}")

    # Dedup while preserving order
    seen = set()
    out = []
    for h in hits:
        if h not in seen:
            seen.add(h)
            out.append(h)
    return out


def staged_paths():
    try:
        out = subprocess.check_output(
            ["git", "diff", "--cached", "--name-only", "-z"],
            stderr=subprocess.DEVNULL,
            text=True,
        )
    except Exception:
        return []
    return [p for p in out.split("\0") if p]


def staged_diff():
    try:
        return subprocess.check_output(
            ["git", "diff", "--cached", "--unified=0"],
            stderr=subprocess.DEVNULL,
            text=True,
            errors="replace",
        )
    except Exception:
        return ""


def deny_secrets(hits, extra=None):
    detail = "; ".join(hits[:12])
    if extra:
        detail = extra + "; " + detail if detail else extra
    emit(
        "deny",
        agent_message=(
            "Blocked: sensitive content detected in staged changes or commit message. "
            "Remove secrets (.env, passwords, API keys, real public IPs/ports, "
            "config.h, *.pem/*.key) before committing/pushing. Hits: "
            + detail
        ),
        user_message="Blocked commit/push: sensitive information detected.",
    )


def check_forbidden_paths(paths):
    bad = []
    for p in paths:
        base = p.rsplit("/", 1)[-1]
        if base == ".env" or (base.startswith(".env.") and base != ".env.example"):
            bad.append(p)
        elif p.endswith("tiny-bot-firmware/include/config.h") or p == "tiny-bot-firmware/include/config.h":
            bad.append(p)
        elif p.endswith(".pem") or p.endswith(".key"):
            bad.append(p)
    return bad


need_secret_scan = False
for seg in git_segments:
    argv = git_argv(seg)
    if argv is None:
        continue
    sub, rest = git_subcommand(argv)
    if sub in ("commit", "push"):
        need_secret_scan = True

if need_secret_scan:
    paths = staged_paths()
    # On push, also scan commits being pushed vs upstream if nothing staged
    diff_text = staged_diff()
    if not diff_text:
        # Compare to upstream if available; else skip path-only checks for push
        try:
            subprocess.check_output(
                ["git", "rev-parse", "--abbrev-ref", "@{upstream}"],
                stderr=subprocess.DEVNULL,
                text=True,
            )
            diff_text = subprocess.check_output(
                ["git", "diff", "--unified=0", "@{upstream}..HEAD"],
                stderr=subprocess.DEVNULL,
                text=True,
                errors="replace",
            )
            if not paths:
                names = subprocess.check_output(
                    ["git", "diff", "--name-only", "-z", "@{upstream}..HEAD"],
                    stderr=subprocess.DEVNULL,
                    text=True,
                )
                paths = [p for p in names.split("\0") if p]
        except Exception:
            pass

    bad_paths = check_forbidden_paths(paths)
    if bad_paths:
        deny_secrets([], extra="forbidden paths: " + ", ".join(bad_paths[:8]))

    hits = scan_text_for_secrets(diff_text, "diff")
    # Commit message from the raw command (before scrubbing)
    msg = extract_commit_message(raw)
    if msg:
        hits.extend(scan_text_for_secrets(msg, "message"))
    if hits:
        deny_secrets(hits)

for seg in git_segments:
    argv = git_argv(seg)
    if argv is None:
        continue
    sub, rest = git_subcommand(argv)
    if sub is None:
        continue
    all_args = [sub] + rest

    if sub == "push" and (
        has_token(rest, ["--force", "--force-with-lease"]) or has_short_f(rest)
    ):
        emit(
            "deny",
            agent_message=(
                "Blocked: force push is not allowed. Push a normal branch update instead, "
                "or ask the user explicitly if history rewrite is required."
            ),
            user_message="Blocked dangerous git force-push.",
        )

    if has_token(all_args, ["--no-verify"]) or (
        sub == "commit" and has_token(rest, ["-n"])
    ):
        emit(
            "deny",
            agent_message=(
                "Blocked: skipping git hooks (--no-verify / commit -n) is not allowed. "
                "Fix hook failures instead of bypassing them."
            ),
            user_message="Blocked --no-verify / commit -n.",
        )

    if sub == "commit":
        msg = extract_commit_message(raw)
        subject = ""
        for line in msg.splitlines():
            if line.strip():
                subject = line.strip()
                break
        if subject and not CONV_SUBJECT_RE.match(subject):
            emit(
                "deny",
                agent_message=(
                    "Blocked: commit subject must use Conventional Commits, e.g. "
                    "'feat: …', 'fix: …', 'docs: …', 'refactor: …', 'chore: …', "
                    "'test: …', 'ci: …', 'build: …', 'perf: …', 'style: …', 'revert: …' "
                    "(optional scope: feat(api): …). Focus the subject on why."
                ),
                user_message="Blocked commit: subject must be Conventional Commits (feat:/fix:/…).",
            )

    # This solo repo allows commit/push on main. Force-push remains blocked above.

    if sub == "reset" and has_token(rest, ["--hard"]):
        emit(
            "deny",
            agent_message=(
                "Blocked: git reset --hard is not allowed. Prefer a non-destructive reset "
                "or ask the user before discarding work."
            ),
            user_message="Blocked git reset --hard.",
        )

    if sub == "clean" and has_short_f(rest):
        emit(
            "deny",
            agent_message=(
                "Blocked: git clean -f is not allowed. List what would be removed with "
                "git clean -n first, and ask the user before deleting untracked files."
            ),
            user_message="Blocked git clean -f.",
        )

COMMIT_CHECKLIST = (
    "Before committing, confirm: (1) English Conventional Commits subject via HEREDOC "
    "(feat:/fix:/docs:/refactor:/chore:/test:/ci:/build:/perf:/style:/revert:, "
    "optional scope like feat(api): …) focused on why; "
    "(2) no .env / secrets / credentials / real public IPs staged; (3) do not use --no-verify; "
    "(4) only include files relevant to this change; "
    "(5) CHANGELOG.md and AGENTS.md/CLAUDE.md updated when required."
)

PR_CHECKLIST = (
    "Before creating the PR, confirm: (1) branch is pushed; (2) PR body has Summary "
    "(1-3 bullets) and Test plan; (3) scope is a single change set; "
    "(4) description matches the real diff; (5) no secrets in the diff."
)

for seg in gh_segments:
    argv = gh_argv(seg) or []
    if len(argv) >= 2 and argv[0] == "pr" and argv[1] == "create":
        emit("allow", agent_message=PR_CHECKLIST)

for seg in git_segments:
    argv = git_argv(seg)
    if argv is None:
        continue
    sub, _ = git_subcommand(argv)
    if sub == "commit":
        emit("allow", agent_message=COMMIT_CHECKLIST)

emit("allow")
PY
