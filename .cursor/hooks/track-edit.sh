#!/usr/bin/env bash
# afterFileEdit: record paths the agent touched this session (for ship-on-stop scope).
set -euo pipefail

input=$(cat)
export TRACK_EDIT_INPUT="$input"

python3 - <<'PY'
import json
import os
import subprocess
import sys

try:
    data = json.loads(os.environ.get("TRACK_EDIT_INPUT") or "{}")
except Exception:
    sys.exit(0)

file_path = (data.get("file_path") or "").strip()
if not file_path:
    sys.exit(0)

try:
    root = subprocess.check_output(
        ["git", "rev-parse", "--show-toplevel"],
        stderr=subprocess.DEVNULL,
        text=True,
    ).strip()
except Exception:
    sys.exit(0)

# Prefer path relative to repo root.
rel = file_path
if file_path.startswith(root + os.sep):
    rel = file_path[len(root) + 1 :]
elif file_path.startswith(root + "/"):
    rel = file_path[len(root) + 1 :]

state_dir = os.path.join(root, ".cursor", "hooks", "state")
os.makedirs(state_dir, exist_ok=True)
state_file = os.path.join(state_dir, "edited-paths")

existing = set()
if os.path.isfile(state_file):
    with open(state_file, encoding="utf-8") as f:
        existing = {ln.strip() for ln in f if ln.strip()}

if rel not in existing:
    with open(state_file, "a", encoding="utf-8") as f:
        f.write(rel + "\n")

print("{}")
PY
