# Data Model: Link Improvements

Authority: PRD FR-84–FR-86; ADR-0031; ADD §§7/9/11.

## Transient Starter fields

- IPC StarterResponse retains existing fields and an optional raw JSON `modes` field. No new stdin fields.
- Starter Reference holds the decoded `Modes` map keyed by supported creation mode. Each ModeContext contains only `Links map[string]string` and `Meta map[string]any` for consumption.
- Raw absence/null means no overrides; `{}` means an empty map. Every present entry must be an object. Omitted/null links/meta inside an object mean no overrides, as for base maps. Unknown extra object fields remain ignored under the existing typed-output trust boundary.
- Recognized keys: `new`, `contribution`, `fork`, independent of offered start_modes. Existing start_modes validation remains unchanged.

## Validation and transition

```text
single subprocess response
  → decode existing fields + raw modes
  → Invoke decodes modes once into Reference.Modes (shape errors: code 37)
  → ValidateResponse checks every mode's keys/values + existing base publication
  → repository resolution and existing mode-selection journey
  → copy base links/meta, overlay selected mode by key
  → Bind effective maps to current provider/source instance
  → existing revision revalidation and atomic create.Run
  → fixed Linker eligibility/inputs on persisted context
  → Linkers publish missing links
  → Importers evaluate current state
```

Invalid modes terminate with code 37 before materialization. Mode names and publication keys are checked in stable sorted order. Error messages identify field/key/mode, never contextual values; Starter-supplied mode names and keys are quoted so control characters cannot reach the terminal raw. A metadata null is a present value and replaces a base value; a null or empty link value is invalid. Merge does not mutate the original response or its maps and does not recursively combine nested metadata.

## Persisted state

No new storage entity or schema version. Only existing work.start_mode, meta, and links persist; neither the modes map nor unselected data is stored. Existing provenance and provider guards remain applicable.

## Eligibility state

Subject.State.Links is the initial Linker-phase map. Known means map membership at storedKey(reg, component.Provider, component.Key), including empty stored values. If storedKey cannot resolve ownership, this guard does not invent a match; existing eligibility/publication rules still apply. Another provider/source-instance value is not the same key. Decisions and projected inputs remain fixed through the Linker loop. Manual link publication and later Importer eligibility do not use the suppression guard.
