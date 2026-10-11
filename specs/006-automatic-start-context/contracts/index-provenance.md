# Contract: `work.db` schema 3 — operational provenance (F5)

Authority: ADD §3 ("operational provenance … is kept by the core in the index"); ADR-0013;
spec FR-025, FR-044, decision D4; `research.md` R10. **Extends** the F1/F2 index
(`works`, `user_version` 1→2). `work.db` remains a projection: nothing reads it as an
authority for a Work's data; the snapshot holds every key and value.

## Table

```sql
-- migration 2 -> 3
CREATE TABLE IF NOT EXISTS work_provenance (
	work_id          TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
	section          TEXT NOT NULL CHECK (section IN ('meta', 'links')),
	key              TEXT NOT NULL,
	source_component TEXT NOT NULL,     -- "<alias>/<name>"
	source_operation TEXT NOT NULL,     -- 'start' (Starter publication) | 'discover' (Linker)
	recorded_at      TEXT NOT NULL,     -- RFC 3339 UTC
	PRIMARY KEY (work_id, section, key)
);
```

`SchemaVersion` = 3. The connection already runs `PRAGMA foreign_keys=ON`, so
`DB.Delete(work)` (used by reconcile for a Work no longer on disk) removes its provenance.

## Writes

| Writer | When | How |
|---|---|---|
| `create.Run` | the commit point, for the Starter's `meta`/`links` | `DB.UpsertWithProvenance(row, entries)` — the `works` upsert and the `start` rows in **one transaction** |
| the extension pipeline | after a Linker's value is persisted to the snapshot | `DB.RecordProvenance(entry)` — `INSERT … ON CONFLICT(work_id, section, key) DO UPDATE` (last source wins) |

`DB.Upsert` keeps its `ON CONFLICT DO UPDATE` form (never `INSERT OR REPLACE`), so
reconcile's upserts never trigger the cascade. A provenance write that fails in the pipeline
does **not** fail the Work or the extension: the snapshot is authoritative; the failure is
visible only under `WORK_DEBUG` (FR-044). A failure of `UpsertWithProvenance` at the commit
point is the existing `materialization-failed` (17), as for `Upsert` today.

## Rebuild and reconcile

- `reconcile.Open` already rebuilds the whole database when the on-disk `user_version` is
  below `SchemaVersion`. Every F4 database is therefore rebuilt once on its first F5 command;
  that is lossless (no provenance existed).
- `DB.Reset` drops `work_provenance` **before** `works`, then re-migrates to 3.
- A rebuild from snapshots restores every Work's row, and (from the snapshots) every meta and
  link value, but **not** provenance: it is operational data that is not part of the canonical
  snapshot (ADR-0013). A rebuilt index simply has no provenance rows for existing Works.
  `Reconcile` (non-rebuild) never touches provenance except by cascade on delete.

## Reads

`DB.Provenance(workID)` returns the rows for a Work ordered by `(section, key)`. F5 has no
user-facing reader (`work status` is F6); tests read it directly.

## Non-goals

No provenance history (only the current source per key), no provenance for the `work`
section, no provenance in `work-state.json`, and no `--json` surface in this slice.
