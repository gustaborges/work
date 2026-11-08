#!/usr/bin/env python3
"""github-issue-linker: finds the issue a Work is about.

  stdin  {"inputs": {"worktree_path": "<abs>", "branch"?: "...",
                     "github.pull_request"?: "<url>"}}
  stdout {"value": "<issue url>"} or {} when no issue can be determined

Order of evidence, first hit wins:
  1. the issues the pull request closes ("Closes #12"), when the Work has one;
  2. an issue number in the branch name (feature/123-title, fix/issue-123).
It never guesses from free text and never links a pull request as an issue.
"""
import json
import re
import sys

import _gh

NAME = "github-issue-linker"
BRANCH_RE = re.compile(r"(?:^|[/_-])(?:issue[-_]?)?(\d+)(?:[-_/]|$)")


def closing_issue(worktree, pr_ref):
    args = ["pr", "view", *([pr_ref] if pr_ref else []), "--json", "closingIssuesReferences"]
    proc = _gh.gh_in(worktree, args)
    if proc is None:
        _gh.fail(NAME, "gh not found on PATH")
    if proc.returncode != 0:
        return None  # no pull request (or no access): fall through to the branch
    for ref in json.loads(proc.stdout).get("closingIssuesReferences") or []:
        if ref.get("url"):
            return ref["url"]
    return None


def issue_from_branch(worktree, branch):
    if not branch:
        return None
    m = BRANCH_RE.search(branch)
    if not m:
        return None
    proc = _gh.gh_in(worktree, ["issue", "view", m.group(1), "--json", "url"])
    if proc is not None and proc.returncode == 0:
        url = json.loads(proc.stdout).get("url") or ""
    else:  # gh missing, not logged in, or not an issue here: ask the public API
        repo = _gh.origin_repo(worktree)
        if not repo:
            return None
        try:
            url = _gh.api(f"repos/{repo[0]}/{repo[1]}/issues/{m.group(1)}").get("html_url") or ""
        except (SystemExit, ValueError):
            return None
    return url if _gh.ISSUE_URL_RE.match(url) else None


def main():
    inputs = _gh.read_stdin(NAME).get("inputs", {})
    worktree = inputs.get("worktree_path")
    if not worktree:
        _gh.fail(NAME, "missing input: worktree_path")

    url = closing_issue(worktree, inputs.get("github.pull_request")) or issue_from_branch(
        worktree, inputs.get("branch")
    )
    json.dump({"value": url} if url else {}, sys.stdout)
    print()


if __name__ == "__main__":
    main()
