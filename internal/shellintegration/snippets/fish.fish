# work shell integration (fish). Add to ~/.config/fish/config.fish:
#   work shell-init fish | source
function work
    set -l _work_cd_file (mktemp)
    env WORK_CD_FILE=$_work_cd_file WORK_SHELL_INTEGRATION=1 command work $argv
    set -l _work_status $status
    if test -s "$_work_cd_file"
        cd -- (cat "$_work_cd_file")
    end
    rm -f "$_work_cd_file"
    return $_work_status
end
