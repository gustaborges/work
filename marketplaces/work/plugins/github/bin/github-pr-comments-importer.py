#!/usr/bin/env python3
"""github-pr-comments-importer: saves a pull request's conversation.

  stdin  {"inputs": {"github.pull_request": "<url>"}, "output_dir": "<dir>"}
  writes context/github/pull-request/comments.md under output_dir

Three sources, each in its own section and oldest first: reviews (verdict and
summary), inline review comments (with file and line) and general comments.
"""
import _gh

NAME = "github-pr-comments-importer"


def section(title, blocks):
    body = "\n".join(blocks) if blocks else "_none_\n"
    return f"## {title}\n\n{body}\n"


def main():
    data = _gh.read_stdin(NAME)
    owner, repo, number = _gh.parse_target(
        NAME, data.get("inputs", {}), "github.pull_request", _gh.PR_URL_RE, "pull request"
    )
    base = f"repos/{owner}/{repo}"
    reviews = [r for r in _gh.paginate(f"{base}/pulls/{number}/reviews")
               if (r.get("body") or "").strip() or r.get("state") != "COMMENTED"]
    inline = _gh.paginate(f"{base}/pulls/{number}/comments")
    general = _gh.paginate(f"{base}/issues/{number}/comments")

    def when(item, key):
        return item.get(key) or ""

    text = f"# Pull request #{number}: conversation\n\n" + section(
        "Reviews",
        [_gh.comment_block(r["state"].lower().replace("_", " "), r.get("user"),
                           when(r, "submitted_at"), r.get("body"))
         for r in sorted(reviews, key=lambda r: when(r, "submitted_at"))],
    ) + section(
        "Inline comments",
        [_gh.comment_block(f"{c['path']}:{c.get('line') or c.get('original_line') or '?'}",
                           c.get("user"), when(c, "created_at"), c.get("body"))
         for c in sorted(inline, key=lambda c: when(c, "created_at"))],
    ) + section(
        "General comments",
        [_gh.comment_block("comment", c.get("user"), when(c, "created_at"), c.get("body"))
         for c in sorted(general, key=lambda c: when(c, "created_at"))],
    )
    _gh.write(data["output_dir"], "context/github/pull-request/comments.md", text)


if __name__ == "__main__":
    main()
