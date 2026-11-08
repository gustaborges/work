"""Shared helpers for the github plugin's components.

Every call goes through the `gh` CLI first (it carries the user's
authentication, including private repositories and GitHub Enterprise logins)
and falls back to the public REST API, so components still work for public
repositories on a machine without `gh`.
"""
import json
import os
import re
import subprocess
import sys
import urllib.request

PR_URL_RE = re.compile(r"^https://github\.com/([^/]+)/([^/]+)/pull/(\d+)/?$")
ISSUE_URL_RE = re.compile(r"^https://github\.com/([^/]+)/([^/]+)/issues/(\d+)/?$")


def fail(component, msg):
    print(f"{component}: {msg}", file=sys.stderr)
    sys.exit(1)


def read_stdin(component):
    try:
        data = json.load(sys.stdin)
    except json.JSONDecodeError:
        fail(component, "invalid input: not a JSON document")
    if not isinstance(data, dict):
        fail(component, "invalid input: not a JSON object")
    return data


def api(endpoint, accept="application/vnd.github+json"):
    """GET a REST endpoint; returns parsed JSON, or text for non-JSON media types."""
    try:
        proc = subprocess.run(
            ["gh", "api", "-H", f"Accept: {accept}", endpoint],
            capture_output=True, text=True, check=True,
        )
        out = proc.stdout
    except (FileNotFoundError, subprocess.CalledProcessError):
        out = _rest(endpoint, accept)
    return json.loads(out) if accept.endswith("json") else out


def _rest(endpoint, accept):
    req = urllib.request.Request(
        f"https://api.github.com/{endpoint}", headers={"Accept": accept}
    )
    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.read().decode("utf-8", "replace")
    except Exception as exc:
        raise SystemExit(f"github request failed for {endpoint}: {exc}")


def paginate(endpoint):
    """Collect every page of a list endpoint (100 items per page)."""
    sep = "&" if "?" in endpoint else "?"
    items, page = [], 1
    while True:
        chunk = api(f"{endpoint}{sep}per_page=100&page={page}")
        items.extend(chunk)
        if len(chunk) < 100:
            return items
        page += 1


def gh_in(cwd, args):
    """Run gh inside a directory; returns CompletedProcess without raising."""
    try:
        return subprocess.run(
            ["gh", *args], cwd=cwd, capture_output=True, text=True
        )
    except FileNotFoundError:
        return None


def write(output_dir, relpath, text):
    path = os.path.join(output_dir, relpath)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def quote(text):
    text = (text or "").strip()
    return "\n".join(f"> {line}" if line else ">" for line in text.splitlines()) or ">"


def parse_target(component, inputs, key, regex, label):
    """Return (owner, repo, number) from a delivered link input."""
    url = str(inputs.get(key, "")).strip()
    m = regex.match(url)
    if not m:
        fail(component, f"input {key} is not a GitHub {label} URL: {url!r}")
    owner, repo, number = m.groups()
    return owner, repo, int(number)


def who(user):
    return "@" + user["login"] if user and user.get("login") else "unknown"


def comment_block(heading, author, when, body):
    return f"### {heading} by {who(author)} ({when})\n\n{(body or '').strip() or '_(no text)_'}\n"
REMOTE_RE = re.compile(r"github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$")


def git_out(cwd, *args):
    try:
        proc = subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)
    except FileNotFoundError:
        return None
    return proc.stdout.strip() if proc.returncode == 0 else None


def origin_repo(cwd):
    """(owner, repo) of the worktree's origin remote, or None when not GitHub."""
    m = REMOTE_RE.search(git_out(cwd, "remote", "get-url", "origin") or "")
    return m.groups() if m else None


def rest_branch_pull_request(cwd):
    """URL of the pull request whose head is the worktree's branch, via REST.

    Fallback for machines where gh is missing or not logged in; open pull
    requests win over closed or merged ones, like `gh pr view`.
    """
    repo = origin_repo(cwd)
    branch = git_out(cwd, "rev-parse", "--abbrev-ref", "HEAD")
    if not repo or not branch or branch == "HEAD":
        return None
    owner, name = repo
    for state in ("open", "all"):
        found = api(f"repos/{owner}/{name}/pulls?head={owner}:{branch}&state={state}&per_page=1")
        if found:
            return found[0]["html_url"]
    return None
