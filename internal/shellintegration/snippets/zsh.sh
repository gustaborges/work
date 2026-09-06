# work shell integration (zsh). Add to ~/.zshrc:
#   eval "$(work shell-init zsh)"
work() {
	local _work_cd_file _work_status
	_work_cd_file="$(mktemp "${TMPDIR:-/tmp}/work_cd.XXXXXX")"
	WORK_CD_FILE="$_work_cd_file" WORK_SHELL_INTEGRATION=1 command work "$@"
	_work_status=$?
	if [ -s "$_work_cd_file" ]; then
		cd -- "$(cat "$_work_cd_file")" || _work_status=$?
	fi
	rm -f "$_work_cd_file"
	return $_work_status
}
