[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidateSet('test', 'release')][string]$Channel,
    [Parameter(Mandatory)][string]$ExpectedSubject
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $version = & "$PSScriptRoot/Get-Version.ps1"
    $test = $Channel -eq 'test'
    $manager = 'dist/signed/manager/SiriModManager.exe'
    $setup = 'dist/signed/setup/SiriModManager_Setup.exe'
    foreach ($binary in @($manager, $setup)) {
        & "$PSScriptRoot/Verify-Signature.ps1" -Path $binary -ExpectedSubject $ExpectedSubject -ExpectedVersion $version -AllowTestCertificate:$test
    }
    $expectedPayload = (Get-Content -Raw dist/unsigned/embedded-manager.sha256).Trim()
    if ((Get-FileHash $manager -Algorithm SHA256).Hash -cne $expectedPayload) {
        throw 'Der veröffentlichte Manager entspricht nicht dem im Setup eingebetteten Manager.'
    }
    if (Test-Path dist/package) { throw 'dist/package existiert bereits. Einen frischen Build verwenden.' }
    New-Item -ItemType Directory -Path dist/package, dist/portable | Out-Null
    $suffix = if ($test) { '_TEST' } else { '' }
    Copy-Item $manager "dist/package/SiriModManager_v${version}${suffix}.exe"
    Copy-Item $setup "dist/package/SiriModManager_Setup_v${version}${suffix}.exe"
    Copy-Item $manager dist/portable/SiriModManager.exe
    Copy-Item manager/app.ico dist/portable/app.ico
    Copy-Item README.md, LICENSE, PRIVACY.md, THIRD_PARTY_NOTICES.md, CODE_SIGNING.md, UNINSTALL.md dist/portable/
    Copy-Item third_party dist/portable/ -Recurse
    if ($test) {
        'Nur zur Prüfung des Signierablaufs. Kein öffentlich vertrauenswürdiger Release.' |
            Set-Content -Encoding utf8 dist/portable/TESTVERSION.txt
    }
    Compress-Archive -Path dist/portable/* -DestinationPath "dist/package/SiriModManager_v${version}${suffix}_Portable.zip"
    Copy-Item LICENSE, THIRD_PARTY_NOTICES.md dist/package/
    Copy-Item third_party/GO-LICENSE.txt dist/package/
    $hashes = Get-ChildItem dist/package -File | Sort-Object Name | ForEach-Object {
        '{0}  {1}' -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $_.Name
    }
    $hashes | Set-Content -Encoding ascii dist/package/SHA256SUMS.txt
    Write-Host "Geprüfte Ausgabe: dist/package ($Channel)"
} finally { Pop-Location }
