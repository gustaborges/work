#!/usr/bin/env python3
"""wsp-repository-locator: finds local clones via the wsp workspace index.

Protocol (specs/004-local-clone-locator/contracts/repository-locator.md):
  stdin  {"repository": {"git_fetch_urls"?, "name"?, "query"?},
          "repository_roots": [...]}
  stdout {"matches": [{"repo_path": "<abs>"}, ...]}

Each accepted field is looked up with `wsp find <text> --json` (clone URLs
are reduced to the "owner/repo" form the index stores); only the
`repositories` hits are used. Results are returned unranked; the core
validates, dedups and asks the user when several remain. `repository_roots`
is ignored: wsp's index, not the filesystem, is the source of truth here.
"""
import json
import os
import re
import subprocess
import sys

# https://host/owner/repo(.git), ssh://git@host/owner/repo and git@host:owner/repo
# all name the same repository; the wsp index records it as "owner/repo".
URL_RE = re.compile(r"^(?:[a-z][a-z0-9+.-]*://)?(?:[^@/]+@)?[^/:]+[:/]([^/]+/[^/]+?)(?:\.git)?/?$", re.I)


def fail(msg):
    print(f"wsp-repository-locator: {msg}", file=sys.stderr)
    sys.exit(1)


def wsp_find(text):
    try:
        proc = subprocess.run(
            ["wsp", "find", text, "--json"], capture_output=True, text=True
        )
    except FileNotFoundError:
        fail("wsp not found on PATH")
    if proc.returncode != 0:
        fail(f"wsp find failed: {proc.stderr.strip() or proc.returncode}")
    try:
        return json.loads(proc.stdout).get("repositories") or []
    except json.JSONDecodeError:
        fail("wsp find returned invalid JSON")


def main():
    try:
        repo = json.load(sys.stdin).get("repository") or {}
    except (json.JSONDecodeError, AttributeError):
        fail("invalid input: not a JSON object")

    terms = []
    for url in repo.get("git_fetch_urls") or []:
        m = URL_RE.match(url)
        terms.append(m.group(1) if m else url)
    for key in ("name", "query"):
        if repo.get(key):
            terms.append(repo[key])

    matches, seen = [], set()
    for term in terms:
        for hit in wsp_find(term):
            path = hit.get("path")
            if not path or not os.path.isabs(path) or path in seen:
                continue
            seen.add(path)
            matches.append({"repo_path": path})

    json.dump({"matches": matches}, sys.stdout)
    print()


if __name__ == "__main__":
    main()
