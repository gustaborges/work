# Data Model: Agent Skills

**Status**: Approved under PRD FR-65–69 and ADR-0031–33; see [governance](governance/README.md). No Work snapshot/index/admin schema changes.

## DeliveredSkill

`name` (inventory-owned portable identity), `tree` (read-only fs.FS), `embedded_digest`. Initial inventory contains only `work-plugin-builder`. Expose Names(), Tree(name), Digest(name); unknown names fail. Inventory includes every intended dot/underscore file, rejects symlinks/special nodes/nested Go modules and non-text assets. Empty directories carry no product content.

## Portable content identity: work-skill-tree-v1

One function accepts embedded, staged and installed trees. Hash SHA-256 over ASCII `work-skill-tree-v1` followed by NUL, then lexically sorted relative UTF-8 POSIX-path records. Reject absolute/escaping/nonportable paths, invalid UTF-8 and non-regular nodes. Exclude only root `.work-skill.json` from payload hashing, but inspect it separately as a regular marker; nested same-name files and every other hidden file participate. Omit empty directories. Include each nonempty directory exactly once.

Each record: unsigned 64-bit big-endian path-byte length; path bytes without leading `./`; one ASCII kind byte (`d` or `f`); unsigned 64-bit big-endian content length; file bytes (`d` has length zero). Paths sort by raw UTF-8 bytes; no locale/case/Unicode normalization. Exclude modes, mtimes and inode identity. String digest is `sha256:` followed by 64 lowercase hexadecimal characters. Test vectors fix framing/order and distinguish path/content concatenation collisions. Unsafe nodes fail even inside otherwise empty directories.

## OwnershipMarker

Root `.work-skill.json`: object `{schema:1, skill:"work-plugin-builder", digest_algorithm:"work-skill-tree-v1", digest:"sha256:…"}`. Validate exact required types, supported schema/algorithm, selected skill and digest shape. Missing/malformed/unsupported/wrong-skill regular marker is foreign, not owned. I/O failure is unreadable; linked/special marker is conflict. Valid marker plus actual payload digest equal recorded means unmodified; equal embedded means current, otherwise outdated. Unequal actual/recorded means modified.

## Destination / TargetObservation

Destination: vendor (`claude|codex`), skill, scope (`global|folder`), initial CWD, selected anchor, canonical anchor, absolute target, canonical coordination parent/target identity. Resolve existing anchor prefix once; safe missing suffixes can be planned without creation. Existing parent filesystem identity plus platform case behavior governs alias detection. Deduplicate exact destination identities and reject overlapping or ambiguous aliases before writes. Result order vendor then skill; lock order canonical target identity independent of output order.

Observation: existence, supported node kinds, coherent payload digest, exact marker bytes/digest, tree/root/ancestor identities and record identity. Payload digest alone is insufficient for recovery: old marker bytes (including foreign bytes) must be preserved/checked. Observe added/deleted files and safe ordinary empty directories in the full-tree recovery fingerprint even though payload digest omits empty directories. Keep exact old tree through a retained rename backup; do not rebuild it from a manifest. Revalidation compares complete observed content and marker, not only mtimes.

## InspectionResult

Required: `vendor`, `skill`, absolute `path`, `state`, `embedded_digest`, nullable `recorded_digest`, nullable `actual_digest`. `error` safe token appears only for conflict/recovery_required/unreadable; no raw I/O causes or user file content. A valid same-skill marker can provide recorded_digest; malformed/foreign markers produce null. Actual digest is present only after complete safe hashing. Missing ancestor → absent, not I/O failure. State precedence: unsafe path/node → conflict; safe owned pending evidence → recovery_required; unreadable inspection → unreadable; otherwise classify marker/payload. Never follow an unsafe node to seek a record.

## InstallPlan / TargetOutcome

Plan holds canonical destinations, immutable embedded digest, observed before-state, allowed action and explicit force flag. Current → unchanged; absent → install; outdated → update; modified/foreign → replacement only if force; conflict/unreadable → fail; recovery_required → locked recovery then replan. A changed observation fails even with force.

Outcome fields: vendor/skill/path; action `installed|updated|unchanged|failed|not-attempted`; commit state `none|rolled-back|committed|cleanup-pending|recovery-required`; safe error token. Cleanup failure after commit is `updated`/`installed` with cleanup-pending, not unchanged or rolled-back. Overall command fails on any error; already committed targets remain successes.

## RecoveryRecord (schema 1)

Persistent sibling record per canonical target, not inside payload. Fields: schema, operation ID (cryptographically random portable token), expected target basename/skill identity, validated stage/backup sibling basenames tied to that operation ID, phase `prepared|committed`, old existence, old complete observation fingerprint (including marker/empty directories), old marker fingerprint, new payload digest and new marker fingerprint. No arbitrary absolute replay paths, external symlinks or caller-provided basenames. Derive parent from the already validated selected destination. Strictly validate fields, identities, naming, kinds and digest matches before recovery; unknown/malformed records are recovery-required and preserved.

All stages/backups are operation-owned siblings. One record per locked destination, one in-flight operation there. Record publication is atomic-file replacement. `prepared` is durable before old move; `committed` is published only after complete new tree/marker are present. Actual tree evidence resolves interruption between a move and record write. Before commit restore exact old backup or original absence; after commit retain verified new target and remove only matching owned backup/stage/record. Unproven combinations stop unchanged with evidence retained. See [state matrix](contracts/skill-installation.md).

## ContractDescriptor / Pin

Immutable sorted copies from validators/identity/semantic conventions and actual IPC types/constants. Canonical deterministic serialization forms fingerprint input, with an explicit descriptor format version. CONTRACT stores its SHA-256 fingerprint, not the skill payload digest. Generated docs derive from the same descriptors. Fixtures cover semantics unrepresentable in descriptors. No source-file hash or second manual validator table.
