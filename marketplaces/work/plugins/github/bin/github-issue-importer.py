#!/usr/bin/env python3
"""github-issue-importer: saves an issue and its discussion.

  stdin  {"inputs": {"github.issue": "<url>"}, "output_dir": "<dir>"}
  writes context/github/issue/issue.md under output_dir

Only eligible when the Work has a github.issue link (declared as a required
input), so a Work without an issue never starts it.
"""
import _gh

NAME = "github-issue-importer"


def main():
    data = _gh.read_stdin(NAME)
    owner, repo, number = _gh.parse_target(
        NAME, data.get("inputs", {}), "github.issue", _gh.ISSUE_URL_RE, "issue"
    )
    issue = _gh.api(f"repos/{owner}/{repo}/issues/{number}")
    if "pull_request" in issue:  # the issues API also serves pull requests
        _gh.fail(NAME, f"{owner}/{repo}#{number} is a pull request, not an issue")
    comments = sorted(
        _gh.paginate(f"repos/{owner}/{repo}/issues/{number}/comments"),
        key=lambda c: c.get("created_at") or "",
    )
    labels = ", ".join(l["name"] for l in issue.get("labels", [])) or "none"
    assignees = ", ".join(_gh.who(a) for a in issue.get("assignees", [])) or "none"
    text = (
        f"# {issue['title']} (#{number})\n\n"
        f"- URL: {issue['html_url']}\n"
        f"- State: {issue['state']}\n"
        f"- Author: {_gh.who(issue.get('user'))}\n"
        f"- Assignees: {assignees}\n"
        f"- Labels: {labels}\n\n"
        f"{(issue.get('body') or '').strip() or '_(no description)_'}\n\n"
        "## Comments\n\n"
        + ("\n".join(
            _gh.comment_block("comment", c.get("user"), c.get("created_at"), c.get("body"))
            for c in comments) or "_none_\n")
    )
    _gh.write(data["output_dir"], "context/github/issue/issue.md", text)


if __name__ == "__main__":
    main()
