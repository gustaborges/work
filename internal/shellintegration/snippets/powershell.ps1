# work shell integration (PowerShell 7+). Add to your profile:
#   Invoke-Expression (& work shell-init powershell | Out-String)
function work {
    $workCdFile = [System.IO.Path]::GetTempFileName()
    $workReal = $env:WORK_REAL_BIN
    if (-not $workReal) {
        $workReal = (Get-Command -CommandType Application work -ErrorAction Stop | Select-Object -First 1).Source
    }
    try {
        $env:WORK_CD_FILE = $workCdFile
        $env:WORK_SHELL_INTEGRATION = "1"
        & $workReal @args
        $code = $LASTEXITCODE
    } finally {
        Remove-Item Env:WORK_CD_FILE -ErrorAction SilentlyContinue
        Remove-Item Env:WORK_SHELL_INTEGRATION -ErrorAction SilentlyContinue
    }
    if ((Test-Path $workCdFile) -and ((Get-Item $workCdFile).Length -gt 0)) {
        Set-Location -LiteralPath (Get-Content $workCdFile -Raw).Trim()
    }
    Remove-Item $workCdFile -ErrorAction SilentlyContinue
    if ($code -ne 0) { $global:LASTEXITCODE = $code }
}
