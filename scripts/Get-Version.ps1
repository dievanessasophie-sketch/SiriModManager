$ErrorActionPreference = 'Stop'
$source = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '../core/model.go')
$match = [regex]::Match($source, '(?m)^const Version = "([0-9]+\.[0-9]+\.[0-9]+)"\r?$')
if (!$match.Success) { throw 'Keine semantische Version in core/model.go.' }
$match.Groups[1].Value
