param(
    [Parameter(Position = 0)]
    [string]$ThreadID,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ExtraArgs
)

if ([string]::IsNullOrWhiteSpace($ThreadID) -or $ExtraArgs.Count -ne 0) {
    [Console]::Error.WriteLine("Usage: read-thread <thread-id>")
    exit 1
}

$dataDir = if (-not [string]::IsNullOrWhiteSpace($env:DISCOBOT_DATA_DIR)) {
    $env:DISCOBOT_DATA_DIR
}
else {
    Join-Path $HOME ".discobot"
}
$threadsDir = if (-not [string]::IsNullOrWhiteSpace($env:DISCOBOT_THREADS_DIR)) {
    $env:DISCOBOT_THREADS_DIR
}
else {
    Join-Path $dataDir "threads"
}
$threadDir = Join-Path $threadsDir $ThreadID
$configPath = Join-Path $threadDir "config.json"
$turnPath = Join-Path $threadDir "turn.json"
$messagesDir = Join-Path $threadDir "messages"

function Read-JsonFile {
    param([string]$Path)

    try {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
            return $null
        }
        $raw = Get-Content -LiteralPath $Path -Raw -ErrorAction Stop
        if ([string]::IsNullOrWhiteSpace($raw)) {
            return $null
        }
        return $raw | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        return $null
    }
}

function Get-JsonPropertyValue {
    param(
        $Object,
        [string]$Name
    )

    if ($null -eq $Object) {
        return $null
    }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }
    return $property.Value
}

function Format-JsonValue {
    param($Value)

    if ($null -eq $Value) {
        return ""
    }
    if ($Value -is [string]) {
        return $Value
    }
    try {
        return $Value | ConvertTo-Json -Compress -Depth 20
    }
    catch {
        return [string]$Value
    }
}

function Format-Part {
    param($Part)

    if ($null -eq $Part) {
        return "[unrecognized part]"
    }

    $partType = [string](Get-JsonPropertyValue $Part "type")
    switch ($partType) {
        "text" {
            return [string](Get-JsonPropertyValue $Part "text").Trim()
        }
        "reasoning" {
            return [string](Get-JsonPropertyValue $Part "text").Trim()
        }
        "tool-call" {
            $toolName = [string](Get-JsonPropertyValue $Part "toolName")
            if ([string]::IsNullOrWhiteSpace($toolName)) {
                $toolName = "tool"
            }
            $toolInput = Format-JsonValue (Get-JsonPropertyValue $Part "input")
            if ([string]::IsNullOrWhiteSpace($toolInput)) {
                return "[tool call] $toolName"
            }
            return "[tool call] $toolName $toolInput"
        }
        "tool-result" {
            $toolName = [string](Get-JsonPropertyValue $Part "toolName")
            if ([string]::IsNullOrWhiteSpace($toolName)) {
                $toolName = "tool"
            }
            $result = "[tool result] $toolName"
            $output = Format-ToolOutput (Get-JsonPropertyValue $Part "output")
            if ([string]::IsNullOrWhiteSpace($output)) {
                return $result
            }
            return $result + [Environment]::NewLine + $output
        }
        "tool-approval-request" {
            $approvalID = [string](Get-JsonPropertyValue $Part "approvalId")
            if ([string]::IsNullOrWhiteSpace($approvalID)) {
                return "[approval request]"
            }
            return "[approval request] $approvalID"
        }
        "tool-approval-response" {
            $approvalID = [string](Get-JsonPropertyValue $Part "approvalId")
            if ([string]::IsNullOrWhiteSpace($approvalID)) {
                return "[approval response]"
            }
            return "[approval response] $approvalID"
        }
        default {
            if ([string]::IsNullOrWhiteSpace($partType)) {
                return "[part]"
            }
            return "[$partType]"
        }
    }
}

function Format-ToolOutput {
    param($Output)

    if ($null -eq $Output) {
        return ""
    }

    $outputType = [string](Get-JsonPropertyValue $Output "type")
    switch ($outputType) {
        "text" {
            return [string](Get-JsonPropertyValue $Output "value").Trim()
        }
        "error-text" {
            return [string](Get-JsonPropertyValue $Output "value").Trim()
        }
        "json" {
            return (Format-JsonValue (Get-JsonPropertyValue $Output "value")).Trim()
        }
        "error-json" {
            return (Format-JsonValue (Get-JsonPropertyValue $Output "value")).Trim()
        }
        "execution-denied" {
            $reason = [string](Get-JsonPropertyValue $Output "reason")
            if ([string]::IsNullOrWhiteSpace($reason)) {
                return "execution denied"
            }
            return "execution denied: $reason"
        }
        "content" {
            $rendered = New-Object System.Collections.Generic.List[string]
            foreach ($item in @(Get-JsonPropertyValue $Output "value")) {
                $itemType = [string](Get-JsonPropertyValue $item "type")
                if ($itemType -eq "text") {
                    $text = [string](Get-JsonPropertyValue $item "text")
                    if (-not [string]::IsNullOrWhiteSpace($text)) {
                        [void]$rendered.Add($text.Trim())
                    }
                    continue
                }
                [void]$rendered.Add((Format-JsonValue $item))
            }
            return [string]::Join([Environment]::NewLine, $rendered)
        }
        default {
            return Format-JsonValue $Output
        }
    }
}

if (-not (Test-Path -LiteralPath $threadDir -PathType Container)) {
    [Console]::Error.WriteLine("Thread not found: $ThreadID")
    [Console]::Error.WriteLine("Expected directory: $threadDir")
    exit 1
}

$config = Read-JsonFile $configPath
$turn = Read-JsonFile $turnPath
$messages = @{}
$children = @{}

if (Test-Path -LiteralPath $messagesDir -PathType Container) {
    Get-ChildItem -LiteralPath $messagesDir -Filter *.json | Sort-Object Name | ForEach-Object {
        $data = Read-JsonFile $_.FullName
        $messageID = [string](Get-JsonPropertyValue $data "id")
        if ([string]::IsNullOrWhiteSpace($messageID)) {
            return
        }
        $messages[$messageID] = $data
        $parentID = [string](Get-JsonPropertyValue $data "parentId")
        if (-not [string]::IsNullOrWhiteSpace($parentID)) {
            $children[$parentID] = $true
        }
    }
}

$leafID = $null
foreach ($candidate in @(
        [string](Get-JsonPropertyValue $config "activeLeafId"),
        [string](Get-JsonPropertyValue $turn "leafMsgId")
    )) {
    if (-not [string]::IsNullOrWhiteSpace($candidate) -and $messages.ContainsKey($candidate)) {
        $leafID = $candidate
        break
    }
}

if ([string]::IsNullOrWhiteSpace($leafID)) {
    $leaves = @()
    foreach ($messageID in $messages.Keys) {
        if ($children.ContainsKey($messageID)) {
            continue
        }
        $messagePath = Join-Path $messagesDir ($messageID + ".json")
        $mtime = [DateTime]::MinValue
        try {
            $mtime = (Get-Item -LiteralPath $messagePath -ErrorAction Stop).LastWriteTimeUtc
        }
        catch {
            $mtime = [DateTime]::MinValue
        }
        $leaves += [pscustomobject]@{
            ID    = $messageID
            MTime = $mtime
        }
    }
    if ($leaves.Count -gt 0) {
        $leafID = ($leaves | Sort-Object MTime, ID -Descending | Select-Object -First 1).ID
    }
}

if ([string]::IsNullOrWhiteSpace($leafID)) {
    Write-Output "# Thread $ThreadID"
    Write-Output "Directory: $threadDir"
    Write-Output ""
    Write-Output "No readable messages found."
    exit 0
}

$history = New-Object System.Collections.Generic.List[object]
$seen = @{}
$current = $leafID
while (-not [string]::IsNullOrWhiteSpace($current) -and -not $seen.ContainsKey($current)) {
    $seen[$current] = $true
    $item = $messages[$current]
    if ($null -eq $item) {
        break
    }
    [void]$history.Add($item)
    $current = [string](Get-JsonPropertyValue $item "parentId")
}
$historyItems = $history.ToArray()
[array]::Reverse($historyItems)

Write-Output "# Thread $ThreadID"
$threadName = [string](Get-JsonPropertyValue $config "name")
if (-not [string]::IsNullOrWhiteSpace($threadName)) {
    Write-Output "Name: $threadName"
}
Write-Output "Directory: $threadDir"
Write-Output ""

foreach ($entry in $historyItems) {
    $message = Get-JsonPropertyValue $entry "message"
    $role = [string](Get-JsonPropertyValue $message "role")
    if ([string]::IsNullOrWhiteSpace($role)) {
        $role = "unknown"
    }
    $role = $role.ToUpperInvariant()
    $createdAt = [string](Get-JsonPropertyValue $message "createdAt")
    $synthetic = ""
    if ([bool](Get-JsonPropertyValue $message "synthetic")) {
        $synthetic = " synthetic"
    }
    Write-Output "## $role $([string](Get-JsonPropertyValue $entry "id"))$synthetic"
    if (-not [string]::IsNullOrWhiteSpace($createdAt)) {
        Write-Output "createdAt: $createdAt"
    }
    foreach ($part in @(Get-JsonPropertyValue $message "parts")) {
        $text = Format-Part $part
        if (-not [string]::IsNullOrWhiteSpace($text)) {
            Write-Output $text
        }
    }
    Write-Output ""
}
