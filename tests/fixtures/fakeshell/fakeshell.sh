#!/usr/bin/env bash
# fakeshell drives the real `work` binary through the bash shell-integration
# snippet and prints the shell's working directory afterwards, so a test can
# assert the session actually moved (FR-022).
#
# Usage: fakeshell.sh <work-binary> <snippet-file> -- <work args...>
set -euo pipefail

work_bin=$1; shift
snippet=$1; shift
[ "$1" = "--" ] && shift

# Define the wrapper function, pointing it at the real binary.
WORK_REAL_BIN=$work_bin
# shellcheck disable=SC1090
source "$snippet"
# The snippet's `command work` must resolve to the real binary: expose it on PATH.
bindir=$(mktemp -d)
ln -s "$work_bin" "$bindir/work"
export PATH="$bindir:$PATH"

work "$@"
status=$?

# Report where the shell ended up.
echo "FAKESHELL_PWD=$PWD"
exit $status
