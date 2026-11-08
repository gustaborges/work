#!/usr/bin/env python3
"""github-pr-linker: finds the pull request of the Work's current branch.

  stdin  {"inputs": {"worktree_path": "<abs>"}}
  stdout {"value": "<pull request url>"} or {} when the branch has no pull request

Asks `gh pr view` from inside the worktree, so the answer follows whatever
repository and branch the Work checked out; without a usable gh it asks the
public REST API for the origin repository's pull request on that branch. No
pull request is not an error.
"""
import json
import sys

import _gh

NAME = "github-pr-linker"


def main():
    worktree = _gh.read_stdin(NAME).get("inputs", {}).get("worktree_path")
    if not worktree:
        _gh.fail(NAME, "missing input: worktree_path")

    proc = _gh.gh_in(worktree, ["pr", "view", "--json", "url"])
    if proc is not None and proc.returncode == 0:
        url = json.loads(proc.stdout).get("url")
    elif proc is not None and "no pull requests found" in proc.stderr.lower():
        url = None
    else:  # gh missing or not logged in: public REST API
        try:
            url = _gh.rest_branch_pull_request(worktree)
        except SystemExit as exc:
            _gh.fail(NAME, f"could not look up the pull request: {exc}")
    json.dump({"value": url} if url else {}, sys.stdout)
    print()


if __name__ == "__main__":
    main()
