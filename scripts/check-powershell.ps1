param(
    [ValidateSet("check", "fix", "format", "lint")]
    [string]$Mode = "check",

    [Parameter(Mandatory = $true)]
    [string]$FileListPath,

    [Parameter(Mandatory = $true)]
    [string]$ProjectRoot
)

$ErrorActionPreference = "Stop"
$script:ResolvedProjectRoot = [System.IO.Path]::GetFullPath($ProjectRoot)

try {
    Import-Module PSScriptAnalyzer -ErrorAction Stop
}
catch {
    Write-Output "PSScriptAnalyzer is required for PowerShell linting and formatting."
    Write-Output "Install it with:"
    Write-Output ""
    Write-Output "  Install-Module PSScriptAnalyzer -Scope CurrentUser -Force"
    exit 1
}

$files = @(Get-Content -LiteralPath $FileListPath -Raw | ConvertFrom-Json)

function Get-RelativePath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $root = $script:ResolvedProjectRoot
    $fullPath = [System.IO.Path]::GetFullPath($Path)

    if (-not $root.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
        $root += [System.IO.Path]::DirectorySeparatorChar
    }

    $rootUri = New-Object System.Uri -ArgumentList $root
    $fileUri = New-Object System.Uri -ArgumentList $fullPath
    $relativePath = [System.Uri]::UnescapeDataString($rootUri.MakeRelativeUri($fileUri).ToString())
    return $relativePath.Replace("/", [System.IO.Path]::DirectorySeparatorChar)
}

function Get-PreferredLineEnding {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Text
    )

    if ($Text.Contains("`r`n")) {
        return "`r`n"
    }
    if ($Text.Contains("`n")) {
        return "`n"
    }
    if ($Text.Contains("`r")) {
        return "`r"
    }
    return [Environment]::NewLine
}

function ConvertTo-ConsistentLineEnding {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Text,

        [Parameter(Mandatory = $true)]
        [string]$LineEnding
    )

    return $Text -replace "`r`n|`n|`r", $LineEnding
}

function Test-PowerShellFileEncoding {
    $invalidFiles = @()

    foreach ($file in $files) {
        $bytes = [System.IO.File]::ReadAllBytes($file)
        $reason = ""

        if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xef -and $bytes[1] -eq 0xbb -and $bytes[2] -eq 0xbf) {
            $reason = "UTF-8 BOM"
        }
        elseif ($bytes.Length -ge 2 -and $bytes[0] -eq 0xff -and $bytes[1] -eq 0xfe) {
            $reason = "UTF-16 LE BOM"
        }
        elseif ($bytes.Length -ge 2 -and $bytes[0] -eq 0xfe -and $bytes[1] -eq 0xff) {
            $reason = "UTF-16 BE BOM"
        }
        elseif ($bytes -contains 0) {
            $reason = "NUL bytes, likely UTF-16"
        }

        if ($reason -ne "") {
            $invalidFiles += [PSCustomObject]@{
                Path   = Get-RelativePath -Path $file
                Reason = $reason
            }
        }
    }

    if ($invalidFiles.Count -gt 0) {
        Write-Output "PowerShell files must be UTF-8 without BOM:"
        $invalidFiles | ForEach-Object { Write-Output "  $($_.Path): $($_.Reason)" }
        Write-Output "Run pnpm format:powershell to rewrite them as UTF-8 without BOM."
        exit 1
    }
}

function Test-PowerShellEncodingUsage {
    $violations = @()
    $writeCommands = @(
        ("Set" + "-Content"),
        ("Out" + "-File"),
        ("Add" + "-Content")
    )
    $forbiddenEncodings = @(
        "Unicode",
        ("BigEndian" + "Unicode"),
        ("UTF" + "32")
    )

    foreach ($file in $files) {
        $lines = [System.IO.File]::ReadAllLines($file)
        for ($index = 0; $index -lt $lines.Length; $index++) {
            $line = $lines[$index]
            if ($line.TrimStart().StartsWith("#")) {
                continue
            }

            foreach ($command in $writeCommands) {
                if ($line -match "(?i)\b$([regex]::Escape($command))\b") {
                    $violations += [PSCustomObject]@{
                        Path    = Get-RelativePath -Path $file
                        Line    = $index + 1
                        Command = $command
                    }
                    break
                }
            }
            foreach ($encoding in $forbiddenEncodings) {
                if ($line -match "(?i)-Encoding\s+$([regex]::Escape($encoding))\b") {
                    $violations += [PSCustomObject]@{
                        Path    = Get-RelativePath -Path $file
                        Line    = $index + 1
                        Command = "-Encoding $encoding"
                    }
                    break
                }
            }
        }
    }

    if ($violations.Count -gt 0) {
        Write-Output "PowerShell writes must use explicit UTF-8 without BOM:"
        $violations | ForEach-Object { Write-Output "  $($_.Path):$($_.Line): $($_.Command)" }
        Write-Output "Use [System.IO.File]::WriteAllText with [System.Text.UTF8Encoding]::new(`$false), or a local Set-Utf8NoBomFile helper."
        exit 1
    }
}

function Invoke-PowerShellFormatter {
    param(
        [Parameter(Mandatory = $true)]
        [string]$File
    )

    $original = [System.IO.File]::ReadAllText($File)
    $lineEnding = Get-PreferredLineEnding -Text $original
    $formatterInput = ConvertTo-ConsistentLineEnding -Text $original -LineEnding $lineEnding

    try {
        $formatted = Invoke-Formatter -ScriptDefinition $formatterInput
    }
    catch {
        throw "Failed to format $(Get-RelativePath -Path $File): $($_.Exception.Message)"
    }
    if ($null -eq $formatted) {
        $formatted = ""
    }
    if ($formatted -is [array]) {
        $formatted = $formatted -join $lineEnding
    }

    return [PSCustomObject]@{
        Original  = $original
        Formatted = $formatted
    }
}

function Test-PowerShellFormatting {
    $unformattedFiles = @()

    foreach ($file in $files) {
        $formatterResult = Invoke-PowerShellFormatter -File $file

        if ($formatterResult.Formatted -ne $formatterResult.Original) {
            $unformattedFiles += Get-RelativePath -Path $file
        }
    }

    if ($unformattedFiles.Count -gt 0) {
        Write-Output "PowerShell files need formatting:"
        $unformattedFiles | ForEach-Object { Write-Output "  $_" }
        Write-Output "Run pnpm format:powershell to update PowerShell formatting."
        exit 1
    }
}

function Format-PowerShellScript {
    $encoding = New-Object System.Text.UTF8Encoding -ArgumentList $false

    foreach ($file in $files) {
        $formatterResult = Invoke-PowerShellFormatter -File $file

        if ($formatterResult.Formatted -ne $formatterResult.Original) {
            [System.IO.File]::WriteAllText($file, $formatterResult.Formatted, $encoding)
            Write-Output "Formatted $(Get-RelativePath -Path $file)"
        }
    }
}

function Repair-PowerShellLint {
    foreach ($file in $files) {
        Invoke-ScriptAnalyzer -Path $file -Fix | Out-Default
    }
}

function Invoke-PowerShellLint {
    $issues = @()
    foreach ($file in $files) {
        $issues += @(Invoke-ScriptAnalyzer -Path $file)
    }

    if ($issues.Count -gt 0) {
        $issues |
            Sort-Object ScriptPath, Line, Column |
            Format-Table -AutoSize
        Write-Output "PSScriptAnalyzer found $($issues.Count) issue(s)."
        exit 1
    }
}

switch ($Mode) {
    "check" {
        Test-PowerShellFileEncoding
        Test-PowerShellEncodingUsage
        Test-PowerShellFormatting
        Invoke-PowerShellLint
    }
    "fix" {
        Repair-PowerShellLint
        Format-PowerShellScript
        Test-PowerShellFileEncoding
        Test-PowerShellEncodingUsage
        Invoke-PowerShellLint
    }
    "format" {
        Format-PowerShellScript
    }
    "lint" {
        Test-PowerShellFileEncoding
        Test-PowerShellEncodingUsage
        Invoke-PowerShellLint
    }
}
