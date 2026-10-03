[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Path,
    [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$ExpectedSubject,
    [Parameter(Mandatory)][ValidatePattern('^[0-9]+\.[0-9]+\.[0-9]+$')][string]$ExpectedVersion,
    [switch]$AllowTestCertificate
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:OS -ne 'Windows_NT') { throw 'Authenticode-Prüfung benötigt Windows.' }
$file = Get-Item -LiteralPath $Path
$signature = Get-AuthenticodeSignature -LiteralPath $file.FullName
if (!$signature.SignerCertificate) { throw "Keine Signatur: $Path" }
if ($signature.SignatureType.ToString() -ne 'Authenticode') { throw 'Eingebettete Authenticode-Signatur erforderlich.' }
if (![string]::Equals($signature.SignerCertificate.Subject, $ExpectedSubject, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Das Signierzertifikat hat nicht den konfigurierten Herausgeber.'
}
$status = $signature.Status.ToString()
if ($status -ne 'Valid') {
    if (!$AllowTestCertificate -or $status -ne 'NotTrusted') {
        throw "Signaturprüfung fehlgeschlagen: $status"
    }
    Write-Warning 'TESTSIGNATUR: Dieses Paket nicht als vertrauenswürdige Veröffentlichung verteilen.'
}
if (!$AllowTestCertificate -and !$signature.TimeStamperCertificate) {
    throw 'Veröffentlichung benötigt einen überprüfbaren Zeitstempel.'
}
$metadata = $file.VersionInfo
if ($metadata.ProductName -cne 'Siri ModManager' -or
    $metadata.ProductVersion -cne $ExpectedVersion -or
    $metadata.FileVersion -cne $ExpectedVersion) {
    throw 'Produktname oder Versionsnummer der signierten Datei stimmt nicht.'
}
Write-Host "Geprüft: $($file.Name), Version $ExpectedVersion, Signaturstatus $status"
