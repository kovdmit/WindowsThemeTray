<#
.SYNOPSIS
    Builds WindowsThemeTray.exe.
.PARAMETER Icons
    Regenerate assets\light.ico and assets\dark.ico before building.
#>
param([switch]$Icons)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if ($Icons) {
    go run ./tools/genicons
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# -H=windowsgui: GUI subsystem, no console window.
# -s -w: strip symbol table and DWARF. -trimpath: no local paths in the binary.
go build -trimpath -ldflags "-H=windowsgui -s -w" -o WindowsThemeTray.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Get-Item WindowsThemeTray.exe | Select-Object Name, Length
