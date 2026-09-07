# ADR-0016: Repository Reference Model and Git Endpoint Semantics

**Status:** Accepted

**Date:** 2026-08-31

**Product Context:** `docs/prd.md` — RF-2, RF-42, RF-44, and RF-49

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 4.1 and 7.1

## Context

After interpreting an argument, a Starter may know a local path, Git endpoints, a name, or only search text. These data have distinct semantics. In particular, a Git URL is not a universal canonical identity: protocols, users, hosts, and paths do not admit safe generic equivalences.

## Decision

The `Repository Reference` v1 is a transient object with independent and optional fields:

```jsonc
{
  "repository": {
    "path": "/home/user/src/payments",
    "git_fetch_urls": ["https://github.com/acme/payments.git", "git@github.com:acme/payments.git"],
    "name": "payments",
    "query": "pay"
  }
}
```

`path` declares already-resolved location and is validated directly by core. `git_fetch_urls` are known endpoints for fetch, not canonical identity. `name` is a known property, though potentially ambiguous. `query` is opaque text for local mechanisms; should not be automatically promoted to `name`.

Locators declare in `accepts` which of the fields `git_fetch_urls`, `name`, and `query` they consume; `path` never appears there. A Locator that consumes URLs compares fetch URLs across all local remotes, not just `origin`, and uses native Git operations instead of reinterpreting `.git/config` or rewrite rules.

Work does not remove protocols, users, or `.git` suffixes, does not infer SSH/HTTPS equivalence, and does not use push URLs. v1 does not introduce typed provider identifiers; they may be considered when concrete consumption exists beyond current fields.

## Consequences

Forge components can publish rich knowledge without forcing provider dependency on Locators; forks clones can match by `upstream`; aliases remain local. In exchange, equivalent endpoints not published by the producer may not match, and there is no universal repository identity in v1.

## Rejected Alternatives

* Singular `remote_url` or canonical Git URL: do not represent all endpoints nor possess safe universal normalization.
* Use only `origin` or push URLs: omits legitimate clones and describes operational destination, not the Git source.
* Treat `name` as global identifier or `query` as `name`: both lose semantics and introduce false matches.
* Typed identifiers in v1: would expand public surface before proven necessity.
