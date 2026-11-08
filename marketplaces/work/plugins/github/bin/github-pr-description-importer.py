#!/usr/bin/env python3
"""github-pr-description-importer: saves a pull request's title and description.

  stdin  {"inputs": {"github.pull_request": "<url>"}, "output_dir": "<dir>"}
  writes context/github/pull-request/description.md under output_dir
"""
import _gh

NAME = "github-pr-description-importer"


def main():
    data = _gh.read_stdin(NAME)
    owner, repo, number = _gh.parse_target(
        NAME, data.get("inputs", {}), "github.pull_request", _gh.PR_URL_RE, "pull request"
    )
    pr = _gh.api(f"repos/{owner}/{repo}/pulls/{number}")
    labels = ", ".join(l["name"] for l in pr.get("labels", [])) or "none"
    head = pr.get("head", {})
    text = (
        f"# {pr['title']} (#{number})\n\n"
        f"- URL: {pr['html_url']}\n"
        f"- State: {'merged' if pr.get('merged_at') else pr['state']}"
        f"{' (draft)' if pr.get('draft') else ''}\n"
        f"- Author: {_gh.who(pr.get('user'))}\n"
        f"- Branch: {head.get('label')} -> {pr.get('base', {}).get('label')}\n"
        f"- Labels: {labels}\n\n"
        f"{(pr.get('body') or '').strip() or '_(no description)_'}\n"
    )
    _gh.write(data["output_dir"], "context/github/pull-request/description.md", text)


if __name__ == "__main__":
    main()
