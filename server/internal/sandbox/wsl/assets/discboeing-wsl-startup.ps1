[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSAvoidUsingPositionalParameters", "", Justification = "Internal startup script keeps Complete calls compact and local.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSReviewUnusedParameter", "", Justification = "Top-level script parameters are consumed by nested functions at runtime.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseApprovedVerbs", "", Justification = "Internal helper names mirror WSL startup actions.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseShouldProcessForStateChangingFunctions", "", Justification = "Internal startup script is invoked non-interactively and cannot support WhatIf.")]
param(
    [ValidateSet("check", "execute", "uninstall")]
    [string]$Mode = "check",

    [Parameter(Mandatory = $true)]
    [string]$VarDiskPath,

    [int]$VarDiskSizeGB = 100,

    [Parameter(Mandatory = $true)]
    [string]$VarDiskLabel,

    [string]$RuntimeID = "",

    [Parameter(Mandatory = $true)]
    [string]$DistroName,

    [Parameter(Mandatory = $true)]
    [string]$InstallDir,

    [Parameter(Mandatory = $true)]
    [string]$StatePath,

    [Parameter(Mandatory = $true)]
    [string]$ImageRef,

    [string]$RootfsArchivePath = "",

    [string]$ResultFile = ""
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

$ExitOK = 0
$ExitActionsRequired = 10
$ExitWSLUnavailable = 42

function Set-Utf8NoBomFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [AllowNull()]
        [string]$Value
    )

    if ($null -eq $Value) {
        $Value = ""
    }

    $encoding = New-Object System.Text.UTF8Encoding -ArgumentList $false
    [System.IO.File]::WriteAllText($Path, $Value, $encoding)
}

function Complete {
    param(
        [int]$ExitCode,
        [string]$Message,
        [string[]]$Actions = @()
    )

    $payload = [ordered]@{
        mode     = $Mode
        exitCode = $ExitCode
        message  = $Message
        actions  = $Actions
    }

    if ($ResultFile.Trim() -ne "") {
        $resultDir = Split-Path -Parent $ResultFile
        if ($resultDir -ne "") {
            New-Item -ItemType Directory -Force -Path $resultDir | Out-Null
        }
        $json = $payload | ConvertTo-Json -Depth 4 -Compress
        Set-Utf8NoBomFile -Path $ResultFile -Value $json
    }

    if ($Message.Trim() -ne "") {
        if ($ExitCode -eq 0) {
            Write-Output $Message
        }
        else {
            Write-Error -Message $Message -ErrorAction Continue
        }
    }
    exit $ExitCode
}

function ConvertTo-WindowsArgument {
    param(
        [AllowNull()]
        [string]$Argument
    )

    if ($null -eq $Argument) {
        $Argument = ""
    }
    if ($Argument -ne "" -and $Argument -notmatch '[\s"]') {
        return $Argument
    }

    $builder = [System.Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $backslashCount = 0

    foreach ($character in $Argument.ToCharArray()) {
        if ($character -eq '\') {
            $backslashCount += 1
            continue
        }

        if ($character -eq '"') {
            if ($backslashCount -gt 0) {
                [void]$builder.Append('\' * ($backslashCount * 2))
            }
            [void]$builder.Append('\')
            [void]$builder.Append('"')
            $backslashCount = 0
            continue
        }

        if ($backslashCount -gt 0) {
            [void]$builder.Append('\' * $backslashCount)
            $backslashCount = 0
        }
        [void]$builder.Append($character)
    }

    if ($backslashCount -gt 0) {
        [void]$builder.Append('\' * ($backslashCount * 2))
    }
    [void]$builder.Append('"')
    return $builder.ToString()
}

function ConvertTo-WindowsArgumentString {
    param([string[]]$Arguments = @())

    return (@($Arguments) | ForEach-Object { ConvertTo-WindowsArgument $_ }) -join " "
}

function Invoke-NativeCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $FilePath
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    if ($null -ne [System.Diagnostics.ProcessStartInfo].GetProperty("ArgumentList")) {
        foreach ($argument in $Arguments) {
            [void]$startInfo.ArgumentList.Add($argument)
        }
    }
    else {
        $startInfo.Arguments = ConvertTo-WindowsArgumentString -Arguments $Arguments
    }

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    if (-not $process.Start()) {
        throw "failed to start native command '$FilePath'"
    }
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()

    $output = (@($stdout, $stderr) | Where-Object { $_ -ne "" }) -join "`n"

    [pscustomobject]@{
        ExitCode = [int]$process.ExitCode
        Output   = $output.Trim()
    }
}

function Invoke-WSL {
    param([string[]]$Arguments = @())
    Invoke-NativeCommand -FilePath "wsl.exe" -Arguments $Arguments
}

function Test-WSLUnavailableMessage {
    param([string]$Message)

    $text = $Message.Replace([string][char]0, "").ToLowerInvariant()
    $normalized = ($text -replace "\s+", " ").Trim()
    return $normalized.Contains("windows subsystem for linux has not been enabled") -or
    $normalized.Contains("windows subsystem for linux is not installed") -or
    $normalized.Contains("windows subsystem for linux optional component is not enabled") -or
    $normalized.Contains("wsl is not installed") -or
    $normalized.Contains("wsl.exe is not available") -or
    $normalized.Contains("wsl.exe --install") -or
    $normalized.Contains("please install wsl") -or
    $normalized.Contains("install wsl") -or
    $normalized.Contains("the specified service does not exist as an installed service")
}

function Assert-WSLAvailable {
    if ($null -eq (Get-Command "wsl.exe" -ErrorAction SilentlyContinue)) {
        Complete $ExitWSLUnavailable "wsl_not_installed: wsl.exe was not found. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
    }

    $status = Invoke-WSL -Arguments @("--status")
    if ($status.ExitCode -eq 0) {
        return
    }
    if (Test-WSLUnavailableMessage $status.Output) {
        Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
    }

    # Older WSL builds may not support --status. Fall back to a list call before
    # treating this as a hard startup-script failure.
    $list = Invoke-WSL -Arguments @("--list", "--quiet")
    if ($list.ExitCode -eq 0) {
        return
    }
    if (Test-WSLUnavailableMessage $list.Output) {
        Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
    }

    throw "wsl.exe --status failed: $($status.Output)"
}

function Get-WSLDistroName {
    $result = Invoke-WSL -Arguments @("--list", "--quiet")
    if ($result.ExitCode -ne 0) {
        if (Test-WSLUnavailableMessage $result.Output) {
            Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
        }
        throw "wsl.exe --list --quiet failed: $($result.Output)"
    }

    $names = New-Object System.Collections.Generic.List[string]
    foreach ($line in $result.Output -split "`r?`n") {
        $name = $line.Replace([string][char]0, "").Trim()
        if ($name -ne "") {
            $names.Add($name)
        }
    }
    return @($names)
}

function Test-DistroInstalled {
    Test-NamedDistroInstalled -Name $DistroName
}

function Test-NamedDistroInstalled {
    param([string]$Name)

    foreach ($distro in @(Get-WSLDistroName)) {
        if ($distro -ieq $Name) {
            return $true
        }
    }
    return $false
}

function Get-RuntimeState {
    if (-not (Test-Path -LiteralPath $StatePath)) {
        return $null
    }
    try {
        return Get-Content -Raw -LiteralPath $StatePath | ConvertFrom-Json
    }
    catch {
        return $null
    }
}

function Get-RuntimeStateInt {
    param(
        [AllowNull()]
        [object]$State,

        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    if ($null -eq $State) {
        return 0
    }
    $property = $State.PSObject.Properties[$Name]
    if ($null -eq $property -or $null -eq $property.Value) {
        return 0
    }
    try {
        return [int]$property.Value
    }
    catch {
        return 0
    }
}

function Get-RuntimeVarDiskSizeGB {
    $state = Get-RuntimeState
    $sizeGB = Get-RuntimeStateInt -State $state -Name "var_disk_size_gb"
    if ($sizeGB -gt 0) {
        return $sizeGB
    }
    return $VarDiskSizeGB
}

function Get-DesiredVarDiskSizeGB {
    $state = Get-RuntimeState
    $sizeGB = Get-RuntimeStateInt -State $state -Name "desired_var_disk_size_gb"
    if ($sizeGB -gt 0) {
        return $sizeGB
    }
    return $VarDiskSizeGB
}

function Get-VarDiskResizeRequestedBy {
    $state = Get-RuntimeState
    if ($null -eq $state) {
        return ""
    }
    $property = $state.PSObject.Properties["var_disk_resize_requested_by"]
    if ($null -eq $property -or $null -eq $property.Value) {
        return ""
    }
    return [string]$property.Value
}

function Test-VarDiskResizePending {
    if (-not (Test-Path -LiteralPath $VarDiskPath)) {
        return $false
    }
    if ((Get-DesiredVarDiskSizeGB) -le (Get-RuntimeVarDiskSizeGB)) {
        return $false
    }
    $requestedBy = Get-VarDiskResizeRequestedBy
    return $requestedBy -eq "" -or $requestedBy -ne $RuntimeID
}

function Test-RuntimeImageCurrent {
    $state = Get-RuntimeState
    if ($null -eq $state) {
        return $false
    }
    return ($state.distro_name -ieq $DistroName) -and ($state.image_ref -eq $ImageRef)
}

function Save-RuntimeState {
    param([int]$AppliedVarDiskSizeGB = 0)

    $stateDir = Split-Path -Parent $StatePath
    if ($stateDir -ne "") {
        New-Item -ItemType Directory -Force -Path $stateDir | Out-Null
    }
    $state = Get-RuntimeState
    $varDiskSizeGB = $AppliedVarDiskSizeGB
    if ($varDiskSizeGB -le 0) {
        $varDiskSizeGB = Get-RuntimeStateInt -State $state -Name "var_disk_size_gb"
    }
    if ($varDiskSizeGB -le 0) {
        $varDiskSizeGB = $VarDiskSizeGB
    }
    $desiredVarDiskSizeGB = Get-RuntimeStateInt -State $state -Name "desired_var_disk_size_gb"
    if ($desiredVarDiskSizeGB -le 0) {
        $desiredVarDiskSizeGB = $VarDiskSizeGB
    }
    $resizeRequestedBy = Get-VarDiskResizeRequestedBy
    if ($AppliedVarDiskSizeGB -gt 0) {
        $resizeRequestedBy = ""
    }
    $payload = [ordered]@{
        version                      = 1
        distro_name                  = $DistroName
        image_ref                    = $ImageRef
        var_disk_size_gb             = $varDiskSizeGB
        desired_var_disk_size_gb     = $desiredVarDiskSizeGB
        var_disk_resize_requested_by = $resizeRequestedBy
        updated_at                   = (Get-Date).ToUniversalTime().ToString("o")
    }
    $json = $payload | ConvertTo-Json -Depth 4
    Set-Utf8NoBomFile -Path $StatePath -Value $json
}

function Remove-RuntimeState {
    Remove-Item -LiteralPath $StatePath -Force -ErrorAction SilentlyContinue
}

function Stop-DistroIfNeeded {
    if (-not (Test-DistroInstalled)) {
        return
    }
    $result = Invoke-WSL -Arguments @("--terminate", $DistroName)
    if ($result.ExitCode -ne 0) {
        $text = $result.Output.ToLowerInvariant()
        if (-not $text.Contains("not found") -and -not $text.Contains("wsl_e_distro_not_found")) {
            throw "terminate WSL distro '$DistroName' failed: $($result.Output)"
        }
    }
}

function Unregister-DistroIfNeeded {
    if (-not (Test-DistroInstalled)) {
        return
    }
    $result = Invoke-WSL -Arguments @("--unregister", $DistroName)
    if ($result.ExitCode -ne 0) {
        $text = $result.Output.ToLowerInvariant()
        if (-not $text.Contains("there is no distribution with the supplied name") -and -not $text.Contains("wsl_e_distro_not_found")) {
            throw "unregister WSL distro '$DistroName' failed: $($result.Output)"
        }
    }
}

function Move-StaleInstallDirAside {
    if (-not (Test-Path -LiteralPath $InstallDir)) {
        return
    }
    $backupDir = "$InstallDir.stale-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
    Move-Item -LiteralPath $InstallDir -Destination $backupDir -Force
}

function Import-Distro {
    if ($RootfsArchivePath.Trim() -eq "") {
        throw "rootfs archive path is required to import or upgrade WSL distro '$DistroName'"
    }
    if (-not (Test-Path -LiteralPath $RootfsArchivePath)) {
        throw "rootfs archive '$RootfsArchivePath' does not exist"
    }
    $parent = Split-Path -Parent $InstallDir
    if ($parent -ne "") {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    Move-StaleInstallDirAside
    $result = Invoke-WSL -Arguments @("--import", $DistroName, $InstallDir, $RootfsArchivePath, "--version", "2")
    if ($result.ExitCode -ne 0) {
        throw "import WSL distro '$DistroName' failed: $($result.Output)"
    }
    Save-RuntimeState
}

function Upgrade-Distro {
    Stop-DistroIfNeeded
    Unregister-DistroIfNeeded
    Move-StaleInstallDirAside
    Remove-RuntimeState
    Import-Distro
}

function Get-DiskDevice {
    $result = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE")
    if ($result.ExitCode -ne 0) {
        if (Test-WSLUnavailableMessage $result.Output) {
            Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
        }
        throw "list WSL block devices failed: $($result.Output)"
    }

    $devices = New-Object System.Collections.Generic.List[string]
    foreach ($line in $result.Output -split "`r?`n") {
        $fields = $line.Trim() -split "\s+"
        if ($fields.Count -eq 2 -and $fields[1] -eq "disk") {
            $devices.Add("/dev/$($fields[0])")
        }
    }
    return @($devices)
}

function Wait-ForSystemDistro {
    $deadline = (Get-Date).AddSeconds(30)
    $lastError = ""
    while ((Get-Date) -lt $deadline) {
        try {
            [void](Get-DiskDevice)
            return
        }
        catch {
            $lastError = $_.Exception.Message
            Start-Sleep -Milliseconds 500
        }
    }
    throw "timed out waiting for WSL system distro after importing '$DistroName': $lastError"
}

function Find-DiskByLabel {
    $result = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=$VarDiskLabel")
    if ($result.ExitCode -eq 2) {
        return ""
    }
    if ($result.ExitCode -ne 0) {
        if (Test-WSLUnavailableMessage $result.Output) {
            Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
        }
        throw "find WSL /var disk by label '$VarDiskLabel' failed: $($result.Output)"
    }

    foreach ($line in $result.Output -split "`r?`n") {
        $device = $line.Trim()
        if ($device -ne "") {
            return $device
        }
    }
    return ""
}

function Read-DiskLabel {
    param([Parameter(Mandatory = $true)][string]$DevicePath)

    $result = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "blkid", "-o", "value", "-s", "LABEL", $DevicePath)
    if ($result.ExitCode -eq 2) {
        return ""
    }
    if ($result.ExitCode -ne 0) {
        throw "read WSL disk label for '$DevicePath' failed: $($result.Output)"
    }
    return $result.Output.Trim()
}

function New-VarDisk {
    $parent = Split-Path -Parent $VarDiskPath
    if ($parent -ne "") {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }

    $diskPartScript = New-TemporaryFile
    try {
        $maximumMB = [int64]$VarDiskSizeGB * 1024
        $diskPartContent = @"
create vdisk file="$VarDiskPath" maximum=$maximumMB type=expandable
exit
"@
        Set-Utf8NoBomFile -Path $diskPartScript -Value $diskPartContent

        $result = Invoke-NativeCommand -FilePath "diskpart.exe" -Arguments @("/s", $diskPartScript.FullName)
        if ($result.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $VarDiskPath)) {
            throw "create WSL /var disk '$VarDiskPath' failed: $($result.Output)"
        }
    }
    finally {
        Remove-Item -LiteralPath $diskPartScript.FullName -Force -ErrorAction SilentlyContinue
    }
}

function Resize-VarDiskHostIfNeeded {
    if (-not (Test-VarDiskResizePending)) {
        return
    }

    $desiredSizeGB = Get-DesiredVarDiskSizeGB
    Stop-DistroIfNeeded
    $unmount = Invoke-WSL -Arguments @("--unmount", $VarDiskPath)
    if ($unmount.ExitCode -ne 0 -and -not (Test-StaleUnmountMessage $unmount.Output)) {
        throw "unmount WSL /var disk '$VarDiskPath' before resize failed: $($unmount.Output)"
    }

    $diskPartScript = New-TemporaryFile
    try {
        $maximumMB = [int64]$desiredSizeGB * 1024
        $diskPartContent = @"
select vdisk file="$VarDiskPath"
expand vdisk maximum=$maximumMB
exit
"@
        Set-Utf8NoBomFile -Path $diskPartScript -Value $diskPartContent

        $result = Invoke-NativeCommand -FilePath "diskpart.exe" -Arguments @("/s", $diskPartScript.FullName)
        if ($result.ExitCode -ne 0) {
            throw "resize WSL /var disk '$VarDiskPath' to ${desiredSizeGB}GB failed: $($result.Output)"
        }
    }
    finally {
        Remove-Item -LiteralPath $diskPartScript.FullName -Force -ErrorAction SilentlyContinue
    }

    Save-RuntimeState -AppliedVarDiskSizeGB $desiredSizeGB
}

function Test-AlreadyMountedMessage {
    param([string]$Message)
    $text = $Message.ToLowerInvariant()
    return $text.Contains("already mounted") -or $text.Contains("already attached")
}

function Test-StaleUnmountMessage {
    param([string]$Message)
    $text = $Message.ToLowerInvariant()
    return $text.Contains("failed to detach") -or
    $text.Contains("invalid argument") -or
    $text.Contains("not mounted") -or
    $text.Contains("not attached") -or
    $text.Contains("cannot find the path specified")
}

function Mount-VarDiskBare {
    $result = Invoke-WSL -Arguments @("--mount", "--vhd", $VarDiskPath, "--bare")
    if ($result.ExitCode -eq 0) {
        return $false
    }
    if (Test-AlreadyMountedMessage $result.Output) {
        return $true
    }
    if (Test-WSLUnavailableMessage $result.Output) {
        Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
    }
    throw "attach WSL /var disk '$VarDiskPath' in bare mode failed: $($result.Output)"
}

function Recover-AlreadyAttachedDisk {
    $unmount = Invoke-WSL -Arguments @("--unmount", $VarDiskPath)
    if ($unmount.ExitCode -ne 0 -and -not (Test-StaleUnmountMessage $unmount.Output)) {
        throw "detach WSL /var disk '$VarDiskPath' during attachment recovery failed: $($unmount.Output)"
    }

    $shutdown = Invoke-WSL -Arguments @("--shutdown")
    if ($shutdown.ExitCode -ne 0) {
        throw "shutdown WSL during /var disk attachment recovery failed: $($shutdown.Output)"
    }
}

function Unmount-VarDisk {
    Assert-WSLAvailable

    $result = Invoke-WSL -Arguments @("--unmount", $VarDiskPath)
    if ($result.ExitCode -ne 0 -and -not (Test-StaleUnmountMessage $result.Output)) {
        throw "unmount WSL /var disk '$VarDiskPath' failed: $($result.Output)"
    }
    Complete $ExitOK "WSL /var disk '$VarDiskPath' is unmounted."
}

function Remove-PathQuietly {
    param([string]$Path)

    if ($Path.Trim() -eq "") {
        return
    }
    Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction SilentlyContinue
}

function Invoke-Uninstall {
    $wslAvailable = $null -ne (Get-Command "wsl.exe" -ErrorAction SilentlyContinue)
    if ($wslAvailable) {
        if (Test-DistroInstalled) {
            Stop-DistroIfNeeded
            Unregister-DistroIfNeeded
        }
        $result = Invoke-WSL -Arguments @("--unmount", $VarDiskPath)
        if ($result.ExitCode -ne 0 -and -not (Test-StaleUnmountMessage $result.Output)) {
            throw "unmount WSL /var disk '$VarDiskPath' failed: $($result.Output)"
        }
    }

    Remove-PathQuietly -Path $InstallDir
    Remove-PathQuietly -Path $VarDiskPath
    Remove-RuntimeState

    Complete $ExitOK "Managed WSL distro '$DistroName' has been uninstalled."
}

function Wait-ForNewDiskDevice {
    param([string[]]$Before)

    $beforeSet = @{}
    foreach ($device in $Before) {
        $beforeSet[$device] = $true
    }

    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        $current = @(Get-DiskDevice)
        $newDevices = @($current | Where-Object { -not $beforeSet.ContainsKey($_) })
        if ($newDevices.Count -eq 1) {
            return $newDevices[0]
        }
        if ($newDevices.Count -gt 1) {
            throw "multiple new WSL disk devices detected after attaching '$VarDiskPath': $($newDevices -join ', ')"
        }
        Start-Sleep -Milliseconds 500
    }

    throw "timed out waiting for WSL disk device after attaching '$VarDiskPath'"
}

function Format-VarDiskIfNeeded {
    param([Parameter(Mandatory = $true)][string]$DevicePath)

    $label = Read-DiskLabel -DevicePath $DevicePath
    if ($label -eq $VarDiskLabel) {
        return
    }
    if ($label -ne "") {
        throw "attached WSL /var disk '$VarDiskPath' appeared as '$DevicePath' with unexpected label '$label'"
    }

    $mkfs = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "mkfs.ext4", "-F", "-L", $VarDiskLabel, $DevicePath)
    if ($mkfs.ExitCode -ne 0) {
        throw "format WSL /var disk '$VarDiskPath' as ext4 on '$DevicePath' failed: $($mkfs.Output)"
    }

    $formattedLabel = Read-DiskLabel -DevicePath $DevicePath
    if ($formattedLabel -ne $VarDiskLabel) {
        throw "formatted WSL /var disk '$VarDiskPath' on '$DevicePath' but label is '$formattedLabel'"
    }
}

function Test-ResizeRequiresFilesystemCheck {
    param([string]$Message)

    $text = $Message.ToLowerInvariant()
    return $text.Contains("please run") -and $text.Contains("e2fsck") -and $text.Contains("first")
}

function Repair-VarFilesystemForResize {
    param([Parameter(Mandatory = $true)][string]$DevicePath)

    Stop-DistroIfNeeded

    $e2fsck = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "e2fsck", "-f", "-p", $DevicePath)
    if ($e2fsck.ExitCode -gt 1) {
        throw "check WSL /var filesystem '$VarDiskPath' on '$DevicePath' failed: $($e2fsck.Output)"
    }
}

function Resize-VarFilesystemIfNeeded {
    param([Parameter(Mandatory = $true)][string]$DevicePath)

    $resize = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "resize2fs", $DevicePath)
    if ($resize.ExitCode -eq 0) {
        return
    }

    if (Test-ResizeRequiresFilesystemCheck -Message $resize.Output) {
        Repair-VarFilesystemForResize -DevicePath $DevicePath
        $resize = Invoke-WSL -Arguments @("--system", "-u", "root", "--", "resize2fs", $DevicePath)
        if ($resize.ExitCode -eq 0) {
            return
        }
    }

    throw "resize WSL /var filesystem '$VarDiskPath' on '$DevicePath' failed: $($resize.Output)"
}

function Invoke-Check {
    Assert-WSLAvailable

    $actions = New-Object System.Collections.Generic.List[string]
    $distroInstalled = Test-DistroInstalled
    if (-not $distroInstalled) {
        $actions.Add("import-distro")
        $actions.Add("ensure-var-disk")
    }
    elseif (-not (Test-RuntimeImageCurrent)) {
        $actions.Add("upgrade-distro")
        $actions.Add("ensure-var-disk")
    }
    else {
        if (Test-VarDiskResizePending) {
            $actions.Add("resize-var-disk")
        }
        $device = Find-DiskByLabel
        if ($device -eq "") {
            if (-not (Test-Path -LiteralPath $VarDiskPath)) {
                $actions.Add("create-var-disk")
            }
            $actions.Add("attach-var-disk")
            $actions.Add("format-var-disk-if-needed")
        }
        elseif ($actions.Contains("resize-var-disk")) {
            $actions.Add("attach-var-disk")
        }
    }

    if ($actions.Count -eq 0) {
        Save-RuntimeState
        Complete $ExitOK "Managed WSL distro '$DistroName' and WSL /var disk '$VarDiskPath' are ready."
    }
    Complete $ExitActionsRequired "WSL startup actions require elevation: $($actions -join ', ')." @($actions)
}

function Invoke-Execute {
    Assert-WSLAvailable

    $distroInstalled = Test-DistroInstalled
    if (-not $distroInstalled) {
        Import-Distro
    }
    elseif (-not (Test-RuntimeImageCurrent)) {
        Upgrade-Distro
    }
    else {
        Save-RuntimeState
    }
    Wait-ForSystemDistro

    if (-not (Test-Path -LiteralPath $VarDiskPath)) {
        New-VarDisk
    }
    Resize-VarDiskHostIfNeeded

    $device = Find-DiskByLabel
    if ($device -ne "") {
        Resize-VarFilesystemIfNeeded -DevicePath $device
        Save-RuntimeState
        Complete $ExitOK "WSL /var disk '$VarDiskPath' is attached as $device."
    }

    $before = @(Get-DiskDevice)
    $alreadyAttached = Mount-VarDiskBare
    if ($alreadyAttached) {
        $device = Find-DiskByLabel
        if ($device -ne "") {
            Resize-VarFilesystemIfNeeded -DevicePath $device
            Save-RuntimeState
            Complete $ExitOK "WSL /var disk '$VarDiskPath' is attached as $device."
        }
        Recover-AlreadyAttachedDisk
        $before = @(Get-DiskDevice)
        [void](Mount-VarDiskBare)
    }

    $device = Find-DiskByLabel
    if ($device -ne "") {
        Resize-VarFilesystemIfNeeded -DevicePath $device
        Save-RuntimeState
        Complete $ExitOK "WSL /var disk '$VarDiskPath' is attached as $device."
    }

    $newDevice = Wait-ForNewDiskDevice -Before $before
    Format-VarDiskIfNeeded -DevicePath $newDevice
    Resize-VarFilesystemIfNeeded -DevicePath $newDevice

    $device = Find-DiskByLabel
    if ($device -eq "") {
        throw "WSL /var disk '$VarDiskPath' was attached and formatted, but label '$VarDiskLabel' is still unavailable."
    }

    Save-RuntimeState
    Complete $ExitOK "WSL /var disk '$VarDiskPath' is attached as $device."
}

try {
    switch ($Mode) {
        "check" { Invoke-Check }
        "execute" { Invoke-Execute }
        "uninstall" { Invoke-Uninstall }
    }
}
catch {
    $message = $_.Exception.Message
    if (Test-WSLUnavailableMessage $message) {
        Complete $ExitWSLUnavailable "wsl_not_installed: WSL is not installed or is not enabled. Install WSL with 'wsl.exe --install', restart Windows if prompted, then restart Discboeing."
    }
    Complete 1 "WSL startup $Mode failed: $message"
}
