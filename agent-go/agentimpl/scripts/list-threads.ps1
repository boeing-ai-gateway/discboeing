if ($args.Count -ne 0) {
	[Console]::Error.WriteLine("Usage: list-threads")
	exit 1
}

$dataDir = if (-not [string]::IsNullOrWhiteSpace($env:DISCOBOT_DATA_DIR)) {
	$env:DISCOBOT_DATA_DIR
} else {
	Join-Path $HOME ".discobot"
}
$threadsDir = if (-not [string]::IsNullOrWhiteSpace($env:DISCOBOT_THREADS_DIR)) {
	$env:DISCOBOT_THREADS_DIR
} else {
	Join-Path $dataDir "threads"
}
$currentThreadID = $env:DISCOBOT_SESSION_ID

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
	} catch {
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

if (-not (Test-Path -LiteralPath $threadsDir -PathType Container)) {
	Write-Output "Threads directory not found: $threadsDir"
	exit 0
}

$items = @()
Get-ChildItem -LiteralPath $threadsDir | Sort-Object Name | ForEach-Object {
	if (-not $_.PSIsContainer -or $_.Name -eq $currentThreadID) {
		return
	}
	$config = Read-JsonFile (Join-Path $_.FullName "config.json")
	$name = [string](Get-JsonPropertyValue $config "name")
	if ([string]::IsNullOrWhiteSpace($name)) {
		$items += $_.Name
		return
	}
	$items += "$($_.Name)`t$($name.Trim())"
}

if ($items.Count -eq 0) {
	Write-Output "No threads found."
	exit 0
}

$items | ForEach-Object { Write-Output $_ }
