# Quickstart / Validation Guide: Daily Cycle — Resume and Archive (F2)

**Feature**: `specs/002-daily-cycle/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F2 end to end. Each maps to spec acceptance scenarios and
Success Criteria. Shapes and codes live in [`contracts/`](./contracts/),
[`data-model.md`](./data-model.md), and [`research.md`](./research.md) — not repeated here.
The F1 quickstart (`specs/001-first-local-work/quickstart.md` S1–S12) MUST still pass
unchanged alongside these (SC-007).

## Prerequisites

- Go 1.26+ and system `git` >= 2.5.
- A POSIX shell or PowerShell 7+ for the shell-integration scenarios.
- No network required for any scenario.

## Build & isolation

```bash
make build
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
export WS="$(mktemp -d)/workspaces"
```

### Fixture: three Works

```bash
src="$(mktemp -d)/demo"; git init -q "$src"
( cd "$src" && git commit -q --allow-empty -m init && git branch -M main )

for s in alpha bravo charlie ; do
  work start "$src" --workspace "$WS" --base main --slug "$s" --prefix '{slug}' --yes
  sleep 1     # distinct last_accessed_at
done
# access order now (most recent first): charlie, bravo, alpha
```

Capture ids for the explicit-target scenarios:

```bash
id_alpha=$(work resume --help >/dev/null; grep -o '"id": *"[^"]*"' "$WS"/in-progress/demo_alpha/work-state.json | cut -d'"' -f4)
# (or read work-state.json directly)
```

---

## S1 — Resume by recency, interactive

**Covers:** US1 #1, #2; SC-001, SC-002, SC-008; FR-002, FR-003, FR-004.

Run `work resume` in a pty harness. **Expect:**

- the picker lists **charlie, bravo, alpha** top-to-bottom, each row:
  ```
  demo  <slug>
    <n> seconds ago • <slug>
  ```
- select **alpha** (least recent) → exit 0; stdout:
  ```
  work: resumed <id_alpha>
  work: path <WS>/in-progress/demo_alpha/worktree
  ```
- `demo_alpha/work-state.json` now has `schema: 2` and a `last_accessed_at` newer than
  bravo's and charlie's; `work.db` `works.last_accessed_at` for alpha equals the snapshot.
- a second `work resume` lists **alpha, charlie, bravo** (SC-002).
- with the fake-shell harness active, the session's cwd is now
  `<WS>/in-progress/demo_alpha/worktree`.

---

## S2 — Resume by explicit id, no list

**Covers:** US1 #3; FR-005.

```bash
work resume "$id_alpha" ; echo $?      # -> 0, no picker
```

**Expect:** no TUI rendered; same two stdout lines; same bump as S1; the recent-access
update is applied exactly as the interactive path.

---

## S3 — Resume error paths

**Covers:** US1 #4, #5; FR-005, FR-007, FR-020; edge cases.

```bash
work resume 01000000000000000000000000 ; echo $?          # unknown id -> 21
work resume < /dev/null ; echo $?                          # non-interactive, no target -> 2
```

Archive `bravo` (S5), then:

```bash
work resume "$id_bravo" ; echo $?                          # archived -> 22
```

**Expect:** each exits with the code shown; `error: <token>: <message>` on stderr; **no**
snapshot or projection write. From inside `demo_alpha/worktree`, `work resume` still lists
`alpha` and selecting it is a valid no-op reposition that still bumps `last_accessed_at`.

---

## S4 — Home TUI reaches both journeys

**Covers:** FR-030.

Run `work` (pty). **Expect:** three rows — "Start a Work", "Resume a Work", "Archive Works".
Arrow to "Resume a Work" + Enter → the S1 picker. Arrow to "Archive Works" + Enter → the S5
picker. Keyboard only.

---

## S5 — Archive: multi-select, confirm, preserve snapshot

**Covers:** US2 #1, #2; SC-003, SC-004; FR-009, FR-010, FR-012, FR-014, FR-015, FR-016.

Run `work archive` (pty):

- all active Works listed, **none checked** (FR-009);
- `Space` on **alpha** and **charlie**; `Enter` → confirmation view naming both and stating
  "2 worktrees will be destroyed; snapshots move to archived/; branches are kept";
- decline (`Esc`, then `Ctrl-C`) → **nothing changed** (SC-004): both worktrees intact, both
  snapshots `status: in-progress`, no `works` row changed;
- re-run, check the same two, confirm.

**Expect after confirm:**

- `<WS>/in-progress/demo_alpha/` and `demo_charlie/` are **gone**;
- `<WS>/archived/<yyyymmdd>-demo_alpha/work-state.json` and `…-demo_charlie/work-state.json`
  exist, are readable, `schema: 2`, `status: "archived"`, `archived_at` set; no `worktree/`;
- `demo_bravo` (not selected) is untouched — worktree present, snapshot `in-progress`;
- `work.db`: alpha & charlie `status = archived`, `worktree_path = ""`; bravo unchanged;
  `work resume` no longer lists alpha or charlie;
- in `$src`: `git branch --list alpha charlie` still shows both branches (FR-014);
- stdout:
  ```
  work: archived <id_alpha>  (<WS>/archived/<yyyymmdd>-demo_alpha)
  work: archived <id_charlie>  (<WS>/archived/<yyyymmdd>-demo_charlie)
  work: archived 2 of 2
  ```

---

## S6 — Archive: explicit targets still confirm

**Covers:** US2 #3, #4; FR-010, FR-011.

```bash
# interactive: explicit id still shows the confirmation view; declining changes nothing
work archive "$id_bravo"          # decline at the confirmation -> exit 20, bravo still active

# non-interactive
work archive "$id_bravo" ; echo $?             # no --yes -> exit 2, nothing archived
work archive "$id_bravo" --yes ; echo $?       # -> 0, archived without any prompt
```

**Expect:** the confirmation is never skipped (FR-010); non-interactive requires `--yes`
(FR-011); with `--yes` the archive runs unattended with a stable exit code.

---

## S7 — Archive: dirty worktree is protected

**Covers:** US2 #5; SC-009; FR-013.

```bash
work start "$src" --workspace "$WS" --base main --slug dirtywork --prefix '{slug}' --yes
echo scratch > "$WS"/in-progress/demo_dirtywork/worktree/UNTRACKED

# non-interactive, no flag:
work archive "$id_dirty" "$id_clean" --yes ; echo $?
```

**Expect:** `demo_clean` is archived; `demo_dirtywork` is **left active** (worktree intact,
snapshot `in-progress`), reported on stderr `note: <id_dirty>: worktree has uncommitted or
untracked changes — left active (use --force-dirty)`; exit 0 (the eligible Work archived).
Re-run with `--force-dirty` → `dirtywork` is archived too. Interactive form: an extra
per-Work prompt appears for `dirtywork`; declining leaves it active and archives the rest.

---

## S8 — Archive: partial batch failure stays consistent

**Covers:** US2 #6; SC-005; FR-017, FR-018.

```bash
# fail the SECOND Work's worktree removal; the first must still be cleanly archived
WORK_FAIL_AT=worktree work archive "$id_x" "$id_y" --yes ; echo $?
```

**Expect:** `x` is fully archived (dir under `archived/`, snapshot + row `archived`); `y` is
**fully active** (worktree intact, snapshot `in-progress`, row `in-progress`) — never
half-moved; the projection matches on-disk reality after the run (FR-018). Repeat with
`WORK_FAIL_AT=move` and `WORK_FAIL_AT=projection`: `move` → `y` fully active; `projection`
→ `y` canonically archived on disk and the run either self-heals the row or exits 24 naming
`y`, and the next `work` command's reconcile brings the row into agreement.

---

## S9 — Archive: already-archived / unknown targets don't break the batch

**Covers:** US2 #7; FR-019.

```bash
work archive "$id_alpha" "$id_bravo" 01000000000000000000000000 --yes ; echo $?
# id_alpha already archived (S5), id_bravo active, last id nonexistent
```

**Expect:** `bravo` is archived; `alpha` reported `note: <id>: already archived`; the bogus
id reported `note: <id>: not found`; neither aborts the run; exit 0; `work: archived 1 of 1`.

---

## S10 — Archiving the current directory's Work repositions the session out

**Covers:** edge case; research R10; ADR-0018 amendment.

```bash
eval "$(work shell-init bash)"     # via harness
cd "$WS"/in-progress/demo_bravo/worktree
work archive "$id_bravo" --yes
# harness asserts: cwd is now "$WS" (workspace root), not a deleted directory
```

Without the hook: exit 0, stderr notes the shell is in a now-deleted directory and names
`$WS` to `cd` to; no false `cd` claim.

---

## S11 — Rebuild the index from snapshots

**Covers:** US3 #1, #3; SC-006; FR-022, FR-025.

```bash
# state: some active, some archived (from S5–S9). Snapshot a fingerprint:
find "$WS" -name work-state.json -exec sha256sum {} + | sort > /tmp/before.txt
sqlite3 "$WORK_HOME/state/work.db" 'SELECT id,status,last_accessed_at FROM works ORDER BY 3 DESC,1 DESC' > /tmp/order-before.txt

rm "$WORK_HOME/state/work.db"
work resume < /dev/null ; true          # triggers the rebuild (then exits 2 for no target — fine)

find "$WS" -name work-state.json -exec sha256sum {} + | sort > /tmp/after.txt
sqlite3 "$WORK_HOME/state/work.db" 'SELECT id,status,last_accessed_at FROM works ORDER BY 3 DESC,1 DESC' > /tmp/order-after.txt
diff /tmp/before.txt /tmp/after.txt        # -> empty: 0 snapshots modified
diff /tmp/order-before.txt /tmp/order-after.txt   # -> empty: identical classification + order
```

**Expect:** both diffs empty. `user_version` is 2. `work resume` / `work archive` behave
exactly as before the DB was deleted.

---

## S12 — Reconcile a stale / inconsistent index

**Covers:** US3 #2; FR-023.

```bash
# corrupt the index without touching snapshots
sqlite3 "$WORK_HOME/state/work.db" "UPDATE works SET last_accessed_at='2000-01-01T00:00:00Z' WHERE id='$id_alpha_archived'"
sqlite3 "$WORK_HOME/state/work.db" "INSERT INTO works (id,slug,status,start_mode,starter,branch,base_branch,repo_name,dir_path,worktree_path,snapshot_path,created_at,last_accessed_at) VALUES ('01GHOSTGHOSTGHOSTGHOSTGHOST','x','in-progress','new','s','x','main','demo','/nope','/nope/worktree','/nope/work-state.json','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')"

work resume < /dev/null ; true             # triggers reconcile
```

**Expect:** the stale `last_accessed_at` is corrected to the snapshot's value; the ghost row
(no snapshot on disk) is dropped; **no** snapshot file is modified (re-check the S11
fingerprint).

---

## S13 — Rebuild skips an unreadable snapshot and diagnoses it

**Covers:** US3 #4; FR-024.

```bash
echo 'not json' > "$WS"/in-progress/demo_bravo/work-state.json   # or a schema-3 blob
rm "$WORK_HOME/state/work.db"
work resume < /dev/null ; true
sqlite3 "$WORK_HOME/state/work.db" "SELECT count(*) FROM works"
```

**Expect:** stderr has `note: snapshot-unreadable: <path>: <reason>`; the `works` table has
every **other** Work indexed; the rebuild did not abort.

---

## Traceability

| Scenario | Spec acceptance / SC |
|---|---|
| S1 | US1 #1, #2 · SC-001, SC-002, SC-008 |
| S2 | US1 #3 |
| S3 | US1 #4, #5 · FR-007, FR-020 |
| S4 | FR-030 |
| S5 | US2 #1, #2 · SC-003, SC-004 |
| S6 | US2 #3, #4 |
| S7 | US2 #5 · SC-009 |
| S8 | US2 #6 · SC-005 · FR-018 |
| S9 | US2 #7 |
| S10 | edge case (current-dir archive) |
| S11 | US3 #1, #3 · SC-006 |
| S12 | US3 #2 |
| S13 | US3 #4 |
| F1 S1–S12 | SC-007 (regression) |
