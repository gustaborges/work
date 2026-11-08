#!/usr/bin/env python3
"""github-pr-diff-importer: saves a pull request's unified diff.

  stdin  {"inputs": {"github.pull_request": "<url>"}, "output_dir": "<dir>"}
  writes context/github/pull-request/changes.diff under output_dir

The diff is what GitHub computes against the base branch today, which is not
necessarily what the Work's worktree contains.
"""
import _gh

NAME = "github-pr-diff-importer"


def main():
    data = _gh.read_stdin(NAME)
    owner, repo, number = _gh.parse_target(
        NAME, data.get("inputs", {}), "github.pull_request", _gh.PR_URL_RE, "pull request"
    )
    diff = _gh.api(f"repos/{owner}/{repo}/pulls/{number}", accept="application/vnd.github.diff")
    _gh.write(data["output_dir"], "context/github/pull-request/changes.diff", diff)


if __name__ == "__main__":
    main()
