#!/usr/bin/env python3
"""github-issue-starter: turns a GitHub issue URL into a Work origin.

  stdin  {"arg": "https://github.com/<owner>/<repo>/issues/<n>"}
  stdout {"repository": {...}, "links": {"github.issue": "<url>"}}

No `base_branch` and no `start_modes`: an issue has no branch, so Work asks
which branch to start from and creates a new Work, with the issue attached.
"""
import json
import sys

import _gh

NAME = "github-issue-starter"


def main():
    arg = str(_gh.read_stdin(NAME).get("arg", "")).strip()
    m = _gh.ISSUE_URL_RE.match(arg)
    if not m:
        _gh.fail(NAME, f"not a GitHub issue URL: {arg!r}")
    owner, repo, number = m.groups()

    try:
        issue = _gh.api(f"repos/{owner}/{repo}/issues/{number}")
    except SystemExit as exc:
        _gh.fail(NAME, f"could not fetch issue {owner}/{repo}#{number}: {exc}")
    if "pull_request" in issue:
        _gh.fail(NAME, f"{owner}/{repo}#{number} is a pull request, not an issue")

    repo_info = issue.get("repository") or _gh.api(f"repos/{owner}/{repo}")
    json.dump(
        {
            "repository": {
                "git_fetch_urls": [repo_info["clone_url"]],
                "name": repo_info["name"],
            },
            "links": {"github.issue": issue.get("html_url") or arg},
        },
        sys.stdout,
    )
    print()


if __name__ == "__main__":
    main()
