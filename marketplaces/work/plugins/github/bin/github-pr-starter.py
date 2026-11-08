#!/usr/bin/env python3
"""github-pr-starter: turns a GitHub pull request URL into a Work origin.

  stdin  {"arg": "https://github.com/<owner>/<repo>/pull/<n>"}
  stdout {"repository": {...}, "base_branch": "<head branch>",
          "start_modes": ["contribution", "fork"],
          "meta": {...}, "links": {...}}

`base_branch` is the PR's head branch: in contribution mode it is the branch
to check out, in fork mode the branch the new Work starts from. The repository
is the *head* repo when the PR comes from a fork (that is where the branch
lives), otherwise the base repo. The pull request itself is published as the
`github.pull_request` link and its number as metadata.
"""
import json
import sys

import _gh

NAME = "github-pr-starter"


def main():
    arg = str(_gh.read_stdin(NAME).get("arg", "")).strip()
    m = _gh.PR_URL_RE.match(arg)
    if not m:
        _gh.fail(NAME, f"not a GitHub pull request URL: {arg!r}")
    owner, repo, number = m.groups()

    try:
        pr = _gh.api(f"repos/{owner}/{repo}/pulls/{number}")
    except SystemExit as exc:
        _gh.fail(NAME, f"could not fetch pull request {owner}/{repo}#{number}: {exc}")
    head = pr.get("head") or {}
    head_repo = head.get("repo")
    branch = head.get("ref")
    if not branch:
        _gh.fail(NAME, "pull request has no head branch")
    if not head_repo:
        _gh.fail(NAME, "head repository was deleted; the branch is no longer available")

    json.dump(
        {
            "repository": {
                "git_fetch_urls": [head_repo["clone_url"]],
                "name": head_repo["name"],
            },
            "base_branch": branch,
            "start_modes": ["contribution", "fork"],
            "meta": {"github.pull_request.number": int(number)},
            "links": {"github.pull_request": pr.get("html_url") or arg},
        },
        sys.stdout,
    )
    print()


if __name__ == "__main__":
    main()
