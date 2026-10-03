[CmdletBinding()]
param(
    [ValidateSet('All', 'Manager', 'Setup')][string]$Target = 'All',
    [string]$WindResPath = $env:WINDRES,
    [string]$ResourcePreprocessor,
    [string]$ManagerPath,
    [switch]$RequireSignedManager,
    [string]$ExpectedSignerSubject,
    [switch]$AllowTestCertificate
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Push-Location $PSScriptRoot
try {
    $version = & "$PSScriptRoot/scripts/Get-Version.ps1"
    if (!$WindResPath) {
        $command = Get-Command windres, x86_64-w64-mingw32-windres -ErrorAction SilentlyContinue | Select-Object -First 1
        if (!$command) { throw 'MinGW-windres fehlt. -WindResPath angeben; siehe README.' }
        $WindResPath = $command.Source
    }
    $WindResPath = (Resolve-Path -LiteralPath $WindResPath).Path
    # windres invokes its C preprocessor; keep the matching tools on PATH.
    $env:PATH = (Split-Path $WindResPath) + [IO.Path]::PathSeparator + $env:PATH
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    New-Item -ItemType Directory -Force -Path dist/unsigned | Out-Null

    function Build-Resources([string]$Package) {
        Push-Location $Package
        try {
            $resource = Get-Content -Raw app.rc
            if (!$resource.Contains('"ProductVersion", "' + $version + '\0"')) {
                throw "Versionsnummer in $Package/app.rc passt nicht zu core.Version."
            }
            $resourceArguments = @('--target=pe-x86-64', '-i', 'app.rc', '-O', 'coff', '-o', 'resources_windows_amd64.syso')
            if ($ResourcePreprocessor) { $resourceArguments += "--preprocessor=$ResourcePreprocessor" }
            & $WindResPath @resourceArguments
            if ($LASTEXITCODE -ne 0) { throw "Ressourcen-Build fehlgeschlagen: $Package" }
        } finally { Pop-Location }
    }

    if ($Target -in @('All', 'Manager')) {
        if ($RequireSignedManager) { throw 'Signierter Build: erst Manager bauen/signieren, dann separat -Target Setup verwenden.' }
        Build-Resources 'manager'
        & go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o dist/unsigned/SiriModManager.exe ./manager
        if ($LASTEXITCODE -ne 0) { throw 'Manager-Build fehlgeschlagen' }
    }
    if ($Target -in @('All', 'Setup')) {
        if (!$ManagerPath) {
            if ($Target -eq 'Setup') { throw '-Target Setup benötigt einen expliziten -ManagerPath.' }
            $ManagerPath = 'dist/unsigned/SiriModManager.exe'
        }
        $ManagerPath = (Resolve-Path -LiteralPath $ManagerPath).Path
        if ($RequireSignedManager) {
            & "$PSScriptRoot/scripts/Verify-Signature.ps1" -Path $ManagerPath -ExpectedSubject $ExpectedSignerSubject -ExpectedVersion $version -AllowTestCertificate:$AllowTestCertificate
        }
        $payloadHash = (Get-FileHash -LiteralPath $ManagerPath -Algorithm SHA256).Hash
        Copy-Item -LiteralPath $ManagerPath -Destination setup/SiriModManager.exe -Force
        if ((Get-FileHash setup/SiriModManager.exe -Algorithm SHA256).Hash -cne $payloadHash) {
            throw 'Eingebetteter Manager stimmt nicht mit der geprüften Datei überein.'
        }
        Build-Resources 'setup'
        & go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o dist/unsigned/SiriModManager_Setup.exe ./setup
        if ($LASTEXITCODE -ne 0) { throw 'Setup-Build fehlgeschlagen' }
        $payloadHash | Set-Content -Encoding ascii dist/unsigned/embedded-manager.sha256
    }
    Write-Host "Build $Target / $version abgeschlossen. Noch nicht signierte Ausgabe: dist/unsigned"
} finally { Pop-Location }
