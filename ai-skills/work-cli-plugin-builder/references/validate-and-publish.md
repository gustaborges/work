# Validate every entry, then publish

There is no marketplace validate command. Discovery is passive; selected installation also checks entrypoints and runtimes. Neither executes components. Runtime behavior needs separate invocation tests. After each source edit use fresh temporary WORK_HOME values: repeated add of an already registered source is unchanged and does not recapture edits.

Parse marketplace.json with a JSON parser and confirm its expected name and complete offered set. Enumerate every plugins[].name, without shell eval or whitespace splitting. For each entry start a fresh isolated home, register the source, inspect `work plugin list --available --json` and assert its exact qualified identity/source before installing exactly `plugin@marketplace`. Ignore unrelated builtin offers. Separate homes permit alternative fallback providers. Validate all entries, including later ones, and report any missing runtime or entrypoint. Never treat the first successful entry as whole-marketplace validation.

POSIX one-entry building block: replace the concrete identity with the parsed catalog entry, and run the whole block for each entry. Keep source edits outside scratch homes. Cleanup failure must fail validation while retaining an earlier command failure. A subshell preserves the caller's WORK_HOME.

```sh
(
  set -eu
  validation_home=$(mktemp -d)
  cleanup() {
    result=$?
    trap - EXIT
    if ! rm -rf -- "$validation_home"; then
      if [ "$result" -eq 0 ]; then result=1; fi
    fi
    exit "$result"
  }
  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  export WORK_HOME="$validation_home"
  work plugin marketplace add .
  work plugin list --available --json
  work plugin add 'example@example-team'
)
```

PowerShell one-entry building block, in a script scope: enumerate parsed entries with ConvertFrom-Json and pass names as single native arguments. Check every native exit explicitly. Preserve preexisting WORK_HOME even on failure; remove only the owned scratch directory. Repeat in fresh homes for every entry and after corrections.

```powershell
& {
  $previous = $env:WORK_HOME
  $validationHome = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N'))
  $failure = $null
  try {
    New-Item -ItemType Directory -Path $validationHome -ErrorAction Stop | Out-Null
    $env:WORK_HOME = $validationHome
    work plugin marketplace add .
    if ($LASTEXITCODE -ne 0) { throw 'registration failed' }
    work plugin list --available --json
    if ($LASTEXITCODE -ne 0) { throw 'listing failed' }
    work plugin add 'example@example-team'
    if ($LASTEXITCODE -ne 0) { throw 'selected installation failed' }
  } catch { $failure = $_ }
  finally {
    $env:WORK_HOME = $previous
    try { Remove-Item -LiteralPath $validationHome -Recurse -Force -ErrorAction Stop }
    catch { if ($null -eq $failure) { $failure = $_ } }
  }
  if ($null -ne $failure) { throw $failure }
}
```

Do not export scratch state into a continuing session. Retain source bytes and restore environment on cancellation as well as errors. Test offline local registration, later-entry failure, corrected-source retry and cleanup failure. Once every entry passes, review the Git diff, commit the marketplace source and publish that repository. Consumers explicitly register the chosen Git source then select a qualified plugin identity. Never claim these checks prove external runtime side effects or prose correctness.
