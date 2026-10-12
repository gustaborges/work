# Feature Specification: On-Demand Context (F6)

**Feature Branch**: `feature/on-demand-context`

**Created**: 2026-10-11

**Status**: Draft

**Input**: User description: "roadmap F6; refer to the PRD and ADD"

**Authorities**: [Roadmap F6](../../docs/roadmap.md#f6--on-demand-context); [PRD](../../docs/prd.md) journeys 7.4–7.6, FR-30, FR-31, FR-38, with FR-27–FR-28, FR-32–FR-35 and FR-50–FR-64; [ADD](../../docs/add/add-0001-work-system-architecture.md) §3, §4, §9–§12; accepted ADR-0012, ADR-0013, ADR-0019, ADR-0020 and ADR-0021. F5's [Semantic Conventions v1](../006-automatic-start-context/contracts/semantic-conventions.md) governs keys and link values.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Current or Named Work Without Changing It (Priority: P1)

A developer checks the Work's identity, state, branch, location and saved relationships using `work status`. From another directory, they inspect a named Work using `work status <work>`. They can also inspect archived Works and request structured output for automation. Checking status never refreshes context or changes activity history.

**Why this priority**: Inspection is useful independently of manual enrichment and establishes the current Work resolution that the other journeys require.

**Independent Test**: Create one active and one archived Work with saved links; inspect each from its directory and by explicit identity, in human and structured formats. Compare persisted state and operational records before and after and verify that no extension runs.

**Acceptance Scenarios**:

1. **Given** a current directory at an active Work's root, its checkout, or a descendant of either, **When** `work status` runs, **Then** it presents that Work's identity, state, branch, active checkout location and every persisted link key and value.
2. **Given** an existing active or archived Work and a current directory outside it, **When** `work status <work>` runs with its identity, **Then** it presents that Work; an archived Work's location is its archived directory.
3. **Given** a current directory within an archived Work's directory, **When** `work status` runs, **Then** it presents the archived Work without requiring a surviving checkout.
4. **Given** either target form, **When** `work status --json` runs, **Then** it returns one structured result containing the same identity, state, branch, location and saved links as the human view, without prompts or presentation controls.
5. **Given** a Work whose saved links are older than an external resource and an installed discovery component, **When** status runs repeatedly, **Then** the saved values are presented unchanged, no component runs, and the snapshot, access history, provenance and configuration are unchanged.
6. **Given** no saved links, **When** status runs, **Then** the human view clearly indicates that there are no links and structured output contains an empty link collection.
7. **Given** a valid canonical snapshot and a missing, stale or unusable global index, **When** status resolves and reads that Work, **Then** it presents canonical data without creating, repairing or updating the index.

---

### User Story 2 - Associate a Link With the Current Work (Priority: P2)

A developer runs `work link`, chooses an available manual Linker, and enters a non-empty value. Work saves the relationship under the selected Linker's semantic key. The direct form `work link <linker> <value>` supports the same operation in scripts. Association uses the value supplied by the developer and executes no plugin.

**Why this priority**: This adds useful context without an external service or executable dependency and makes link values available to later manual imports.

**Independent Test**: Install a package declaring a manual Linker and create an active Work. Associate a value interactively and then replace it through the direct command. Inspect the saved link and provenance, confirm unrelated state is preserved, and verify that no executable ran.

**Acceptance Scenarios**:

1. **Given** enabled packages containing manual Linkers, discovery-only Linkers, and Importers, **When** `work link` offers choices, **Then** only enabled Linkers declaring manual association appear, with their manual display text and distinguishable component identities.
2. **Given** a selected manual Linker and no existing value for its key, **When** the developer accepts a non-empty value, **Then** exactly that key is saved with exactly that value and its current provenance identifies the selected component, manual association and recording time.
3. **Given** a value previously published by a Starter, discovery, or another manual provider, **When** a manual Linker for the same semantic key saves a new value, **Then** the new value replaces the old one and becomes the current source; unrelated links and metadata are preserved.
4. **Given** two manual Linkers sharing one semantic key, **When** choices are offered, **Then** both providers remain selectable and associating through either updates the same relationship.
5. **Given** a manual Linker with discovery inputs that are absent, **When** the developer associates a value manually, **Then** association remains available because no discovery operation or input-dependent subprocess is requested.
6. **Given** `work link <linker> <value>` with a valid manual component, **When** it runs with non-interactive input, **Then** the association succeeds without selection or value collection and has the same effect as the interactive journey.
7. **Given** an empty value entered interactively, **When** validation runs, **Then** the field displays one recoverable error and no mutation occurs; a later valid entry can complete the journey. An empty direct value fails without mutation.
8. **Given** cancellation before the association is committed, **When** the developer exits selection or value collection, **Then** no link, provenance or access history changes.

---

### User Story 3 - Import Artifacts Into the Current Work on Demand (Priority: P2)

A developer runs `work import`, selects an eligible manual Importer, and receives artifacts beside the checkout. A required link can come from the Starter, automatic discovery, or a previous manual association. The direct form `work import <importer>` runs the same operation without selection. Work checks the complete output before incorporating it.

**Why this priority**: This completes F6's enrichment journey while preserving the artifact guarantees already delivered by F5.

**Independent Test**: Create a Work with the inputs required by a manual Importer. Import two nested artifacts, repeat with a collision, then use an Importer that fails or produces unsafe content. Verify complete incorporation on success and no artifact changes on refusal or failure.

**Acceptance Scenarios**:

1. **Given** enabled packages containing manual Importers, automatic-only Importers and manual Importers missing required data, **When** `work import` offers choices, **Then** only enabled Importers declaring manual availability with all required inputs present appear.
2. **Given** a manual Importer restricted to a different Starter for automatic activation, **When** its required manual inputs are present, **Then** it is available; event subscriptions and Starter filters do not restrict manual activation.
3. **Given** an Importer with required and optional inputs, **When** it runs, **Then** it receives only the declared inputs that are present, with their saved values and types, plus an exclusive output location; missing optional inputs are omitted.
4. **Given** a manually associated link required by an Importer, **When** `work import <importer>` runs afterwards, **Then** it consumes the current saved value without triggering discovery.
5. **Given** valid output with nested files and no collision, **When** the Importer completes successfully, **Then** every artifact is incorporated at its relative path beneath the Work directory, beside the checkout, and its temporary output is removed.
6. **Given** output containing one new file and one colliding file, **When** incorporation is considered, **Then** the command fails, neither file is incorporated, existing content is unchanged, and temporary output is removed.
7. **Given** a process failure, invalid response, unsafe output, interruption before commitment, or recoverable failure during incorporation, **When** the command ends, **Then** it reports failure or cancellation, contributes no artifacts from that execution, preserves saved links and metadata, and removes temporary output and any partial incorporation.
8. **Given** an Importer that successfully produces no artifacts, **When** it completes, **Then** the command reports success without changing the Work's content.
9. **Given** imported artifacts, **When** the Work is later archived through the existing archive journey, **Then** those artifacts remain preserved with its snapshot.

---

### User Story 4 - Resolve Missing Context and Unavailable Operations Clearly (Priority: P2)

A developer receives actionable guidance when the current directory is outside a Work, a component is unavailable, required data is absent, or a selection is ambiguous. Fully specified commands work without interaction; incomplete automation fails before making changes.

**Why this priority**: These boundaries make the three journeys predictable and prevent accidental enrichment of another Work.

**Independent Test**: Exercise all three commands outside a Work, use missing and ambiguous component names inside one, omit required values with non-interactive input, and request an Importer whose required link is absent. Confirm actionable failures, no extension execution and no mutation.

**Acceptance Scenarios**:

1. **Given** a directory outside any Work, **When** `work status`, `work link` or `work import` runs without a resolvable context, **Then** it fails with guidance to enter a Work; status also explains its explicit target form. Nothing changes.
2. **Given** an unknown explicit Work identity or an unreadable or invalid target snapshot, **When** status runs, **Then** it fails once with an actionable diagnostic and does not substitute another Work or repair state.
3. **Given** an explicit component that is unknown, disabled, of the wrong role, or lacks manual availability, **When** a manual command runs, **Then** it fails without running a component or changing the Work.
4. **Given** an explicit Importer missing required inputs, **When** import runs, **Then** it reports the missing input keys without exposing values, and does not start the Importer.
5. **Given** two enabled manual components with the same name, **When** the unqualified name is supplied directly, **Then** the command fails with guidance listing qualified identities; supplying a qualified identity chooses exactly that component.
6. **Given** no available manual Linker or no eligible manual Importer, **When** an interactive command runs, **Then** it reports the absence and an applicable next step without opening an empty selector or mutating state.
7. **Given** non-interactive input with a missing component or link value, **When** the command runs, **Then** it fails with usage guidance and opens no interactive control, even if only one candidate exists.
8. **Given** a directory inside an archived Work, **When** link or import runs, **Then** it fails with guidance that enrichment requires an active Work, leaving the archive unchanged.

### Edge Cases

- A current directory is a nested checkout directory, a Work artifact directory, or an archived directory; resolution must identify the enclosing Work without guessing from a repository remote, branch or recency.
- An explicit status target differs from the current Work; the explicit identity wins.
- A Work's checkout is missing or inaccessible; status still reports its canonical identity, state and recorded applicable location, without repairing it or claiming the checkout is usable.
- The global index is missing or has stale link data; status uses the snapshot and performs no index maintenance.
- A link value contains spaces or characters meaningful to a shell; a correctly supplied argument is preserved exactly. Empty means zero-length text, consistent with Semantic Conventions v1.
- An optional input is absent versus present with a valid null metadata value; absence is omitted and present data retains its type.
- A component or required input becomes unavailable after selection; eligibility is validated again before the mutation or execution, with no substitute provider selected silently.
- Another operation changes a different link before manual association commits; the association preserves the newer unrelated data and changes only its own key.
- The Work is archived while a manual journey is collecting values or running an Importer; the operation must not modify an archived or stale destination and must clean up its temporary output.
- Output attempts to overwrite the canonical snapshot, enter the checkout, traverse outside the Work, follow a symbolic link, use unsupported file types, or collide under the destination filesystem's case rules; the entire import is refused.
- Existing artifact directories can receive new files when every destination is safe; repeated identical imports still fail if they would overwrite existing files.
- A cancellation arrives during execution or incorporation; the command terminates the selected operation and removes uncommitted effects, preserving earlier successful commands.

## Requirements *(mandatory)*

### Functional Requirements

#### Work context and inspection

- **FR-001**: `work status [work]` MUST resolve an explicit Work identity when supplied; otherwise it MUST use the Work enclosing the current directory. An explicit identity takes precedence over current context.
- **FR-002**: Current Work resolution MUST support the Work root, checkout and their descendants, including artifact directories; status MUST also support archived Work directories. It MUST NOT select a Work by recency or repository similarity.
- **FR-003**: Status MUST present canonical Work identity, lifecycle state, branch, applicable absolute location, and every persisted link's semantic key and value. Active location means the checkout path; archived location means the archived Work directory.
- **FR-004**: Status MUST support `--json`, producing one documented, stable structured result with the same information as the human view. Empty links MUST be represented explicitly.
- **FR-005**: Status MUST be strictly read-only: no extension execution, discovery, snapshot writes, access updates, link timestamp or provenance changes, configuration changes, bootstrap, or index creation, repair or reconciliation. Its success MUST NOT depend on a healthy global index when a valid target snapshot exists.
- **FR-006**: Missing context, an unknown identity, and an unreadable or invalid canonical snapshot MUST produce actionable failures without mutation, guessed fallback or interactive Work selection.

#### Manual component selection

- **FR-007**: `work link [linker] [value]` and `work import [importer]` MUST operate on the current active Work. They MUST reject missing context and archived Works before mutation or extension execution.
- **FR-008**: Interactive choices MUST be derived from enabled installed packages and the requested role's declared manual availability, showing the declared manual display name and description while keeping component identities distinguishable. Importer choices MUST additionally satisfy all required inputs.
- **FR-009**: Qualified component identities MUST use the existing package-alias/component-name convention. An unqualified explicit name MUST resolve uniquely among enabled manual components of the requested role or fail with qualified alternatives; it MUST NOT cause a hidden choice or fallback.
- **FR-010**: Explicit components MUST skip component selection while preserving all role, enablement, manual availability, input and Work-state validations. These validations MUST be repeated before commitment or execution when selection and execution are separated.
- **FR-011**: Manual activation MUST NOT require an event subscription, automatic discovery enablement, or a match against automatic Starter filters. A manual Linker MUST NOT require its discovery inputs to be present.
- **FR-012**: No eligible candidate MUST produce an actionable failure without an empty selector. Missing-input diagnostics MUST identify required keys without exposing their values; when no manual component is installed or enabled, guidance MUST explain that prerequisite.

#### Manual link association

- **FR-013**: Manual association MUST accept a non-empty text value, prompting only for omitted values in an interactive journey, and preserve accepted values exactly under the selected Linker's declared key according to Semantic Conventions v1.
- **FR-014**: Association MUST execute no plugin, perform no discovery, and update only the selected link and its corresponding operational provenance. It MUST NOT modify metadata, unrelated links, core lifecycle fields or recent access.
- **FR-015**: Association MUST use the existing single-current-value upsert semantics: the last committed publication wins, regardless of whether the earlier source was a Starter, discovery or another manual provider. Multiple Linkers declaring the same key MUST remain valid alternatives.
- **FR-016**: Link updates MUST publish a complete valid canonical snapshot atomically and preserve unrelated concurrent changes. Provenance MUST identify the selected qualified component, manual association and recording time using the existing operational provenance model.
- **FR-017**: Invalid values, cancellation before commitment, or a failed canonical update MUST leave links and provenance unchanged and MUST return a failure or cancellation result. A successful association MUST be visible immediately through status and to subsequent eligible Importers.

#### Manual import

- **FR-018**: A selected Importer MUST run only when all declared required inputs resolve from current Work facts, saved links or metadata. Optional absent inputs MUST NOT block eligibility and MUST be omitted; only declared present inputs, preserving their values and types, and the exclusive output location may be supplied.
- **FR-019**: Manual import MUST use the same public execution, input and output contracts and artifact safety rules as F5. It MUST invoke exactly the selected Importer's import operation without invoking Linkers, Starters, other Importers, or publishing a lifecycle event.
- **FR-020**: Each execution MUST receive a unique temporary output location. Before accepting any artifacts, Work MUST validate the complete output for collisions, reserved locations, unsafe traversal, symbolic links and unsupported file types. Destination filesystem name equivalence and concurrent collisions MUST also be enforced during incorporation; any detected collision MUST cause the entire execution to be rolled back before success is reported.
- **FR-021**: Safe artifacts MUST be incorporated with their relative paths beneath the Work directory, beside its checkout. Existing files MUST never be overwritten; existing directories may be extended only when every destination is safe.
- **FR-022**: Refusal of any artifact or failure during incorporation MUST leave the Work's artifact content as it was before that execution, removing any partial incorporation. Temporary output MUST be removed on success, empty success, failure, refusal and cancellation.
- **FR-023**: Manual execution or output failure MUST return a command failure with the Work, component and operation identified. It MUST NOT be treated as successful automatic enrichment with a warning. Saved links, metadata, lifecycle state and recent access MUST remain unchanged.
- **FR-024**: Work MUST impose no execution timeout of its own. User interruption MUST terminate the selected execution, discard uncommitted output and return cancellation; earlier completed enrichment commands MUST remain intact.
- **FR-025**: Manual import MUST preserve the existing archive guarantee that incorporated artifacts remain alongside the snapshot after archiving. A Work that becomes archived before import commitment MUST not receive new artifacts.

#### Interaction, automation and compatibility

- **FR-026**: Interactive journeys MUST collect only omitted choices and values, preserve accepted receipts, keep recoverable value errors in the active control, support cancellation and adapt to the supported terminal dimensions through the current presentation conventions.
- **FR-027**: Non-interactive commands MUST never open interactive controls. Import requires an explicit Importer; link requires an explicit Linker and non-empty value. Missing arguments MUST fail with actionable usage; confirmation shortcuts MUST NOT select components or invent values.
- **FR-028**: Status results and stable mutation results MUST use stdout; interactive presentation, progress and diagnostics MUST use the configured interface channel. Success, failure and cancellation MUST have documented stable results and exit behavior, and terminal failures MUST be rendered once.
- **FR-029**: Presentation MUST remain understandable without color; redirected output, non-empty `NO_COLOR` and `TERM=dumb` MUST receive no styling or interactive control sequences. Diagnostics and accepted receipts MUST avoid exposing link values, metadata values or artifact content unnecessarily; requested status results may present persisted links.
- **FR-030**: The three commands MUST be discoverable through the existing grouped help using their public grammar, without introducing command aliases or a new navigation hub.
- **FR-031**: F6 MUST preserve all previously delivered start, resume, archive, plugin, repository and convention journeys, including automatic context behavior and existing command contracts. It MUST NOT introduce another canonical state or eligibility model.

### Key Entities *(include if feature involves data)*

- **Work context**: The uniquely resolved Work enclosing the current directory or named by an explicit identity for status; includes lifecycle state, canonical location and checkout location where applicable.
- **Status view**: A read-only representation of canonical identity, state, branch, applicable location and persisted semantic links, available as human-readable or structured output.
- **Manual component**: An installed Linker or Importer with declared manual availability, package-qualified identity and presentation text; Importer eligibility also depends on its declared required inputs.
- **Link association**: A single current text value under a semantic key, with publication independent of provider identity; manual updates replace the previous current value.
- **Operational provenance**: The current component, operation and recording time for a published link; remains separate from semantic state and follows F5's recovery limitations.
- **Import execution**: One selected Importer, its resolved declared inputs, exclusive temporary output and final all-or-nothing incorporation outcome.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In all acceptance cases covering active and archived Works, current-directory and explicit identity resolution, and human and structured views, status presents exactly the canonical identity, state, branch, applicable location and saved links. Repeating each read 20 times causes zero persisted state changes and zero extension executions.
- **SC-002**: For both interactive and direct association, 100% of accepted values appear unchanged under the selected semantic key, replace an earlier value when present, and are usable by a subsequent manual import. No plugin executes during association.
- **SC-003**: Across the eligibility cases—enabled/disabled, manual/automatic-only, missing/present required inputs, absent optional inputs, Starter filters and duplicate component names—100% of offered choices are valid for the requested manual operation, and invalid explicit requests produce zero executions and zero mutations.
- **SC-004**: Every successful non-empty import incorporates all expected artifacts, and every collision, unsafe-output, execution-failure, interruption and recoverable incorporation-failure case contributes zero artifacts. All cases leave zero temporary output directories after normal command termination.
- **SC-005**: All incomplete non-interactive requests fail with an actionable result and zero interactive controls; fully specified association, import and status requests complete without interaction and produce their documented stable output and exit results.
- **SC-006**: In supported terminal sizes and color modes, all manual journeys can be completed using only the keyboard; cancelled and rejected attempts cause zero mutations and leave no rejected value or expanded selector in terminal history.
- **SC-007**: The F6 demonstration—inspect a Work, associate a link, import artifacts using that link, inspect again, then repeat missing-context failures—completes with correct results and no manual snapshot editing. Every automated demonstration from the previous slices remains successful.

## Assumptions

- F5 is the prerequisite for installed manual declarations, semantic key validation, input resolution, canonical link persistence, operational provenance and safe artifact incorporation. F6 exposes manual journeys over those contracts.
- Explicit `work` targets use the existing opaque Work identity established by the daily-cycle commands. Additional slug, branch, path or partial-identity target syntax is outside this slice.
- Status can inspect active and archived Works. Manual enrichment is limited to the current active Work because the declared input surface can refer to a live checkout; enriching archives is not part of this slice.
- Manual Linker association depends on enablement and manual availability, not discovery inputs. Those inputs describe the separate discovery operation. Non-empty text follows the existing zero-length check without trimming or imposing a universal URL format.
- A manual Importer's event and Starter restrictions govern automatic activation only. Its manual activation depends on manual availability and current required inputs.
- Operational provenance retains F5's index-only, current-source model: rebuilding the index cannot restore provenance history. A failure to record operational provenance after a successful canonical link commitment follows F5's diagnostic behavior and does not undo or misreport the saved association.
- A missing index must not cause status to write recovery data. Resolution can consult existing information, but the canonical snapshot remains the authority for displayed values.
- Collision validation retains F5 semantics: the complete output is checked before incorporation, and native filesystem creation checks are the final protection against destination aliases and races. Any such failure removes the execution's partial output before commitment. Case-distinct names remain valid on filesystems that distinguish them.
- Integrity guarantees cover core-controlled state publication and incorporation, including recoverable failures and explicit cancellation. Staging retains F5's trust boundary and does not provide a plugin security sandbox or new guarantees for abrupt machine failure.
- F7 plugin maintenance, new automatic discovery triggers, link removal, bulk enrichment, explicit Work targets for link/import, new official integrations and external-resource navigation are outside F6.
- The constitution file remains an unratified template. The PRD, accepted ADRs, ADD and roadmap cross-cutting gates govern this specification. Current PRD and accepted ADR-0021 govern full-screen interaction where the roadmap retains older inline wording.
