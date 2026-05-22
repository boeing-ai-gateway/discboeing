#Requires -RunAsAdministrator
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSAvoidUsingWriteHost", "", Justification = "Smoke test is intentionally interactive and operator-facing.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseShouldProcessForStateChangingFunctions", "", Justification = "Smoke test already checkpoints and restores the disposable VM.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseUsingScopeModifierInNewRunspaces", "", Justification = "Invoke-Command script blocks receive values through ArgumentList parameters.")]
[CmdletBinding()]
param(
    [string]$VMName = "Win11-20260511-135937-5122",

    [pscredential]$GuestCredential,

    [string]$GuestUsername,

    [securestring]$GuestPassword,

    [Parameter(Mandatory = $true)]
    [string]$RootfsArchivePath,

    [string]$StartupScriptPath = (Join-Path $PSScriptRoot "..\src-tauri\resources\wsl\discobot-wsl-startup.ps1"),

    [string]$HostOutputDir = (Join-Path $PSScriptRoot "..\build\wsl-startup-hyperv-test"),

    [string]$GuestWorkDir = "C:\DiscobotWslStartupTest",

    [switch]$TestUpgrade,

    [switch]$NoRestore,

    [switch]$KeepCheckpoint
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

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

function Resolve-RequiredPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    $resolved = Resolve-Path -LiteralPath $Path -ErrorAction SilentlyContinue
    if ($null -eq $resolved) {
        throw "$Description not found: $Path"
    }
    return $resolved.ProviderPath
}

function Wait-VMHeartbeat {
    param([string]$Name)

    $deadline = (Get-Date).AddMinutes(5)
    while ((Get-Date) -lt $deadline) {
        $vm = Get-VM -Name $Name
        if ($vm.State -eq "Running") {
            $heartbeat = Get-VMIntegrationService -VMName $Name -Name "Heartbeat" -ErrorAction SilentlyContinue
            if ($null -eq $heartbeat -or $heartbeat.PrimaryStatusDescription -match "OK") {
                return
            }
        }
        Start-Sleep -Seconds 2
    }
    throw "Timed out waiting for VM heartbeat: $Name"
}

function New-GuestSession {
    param(
        [string]$Name,
        [pscredential]$Credential
    )

    $deadline = (Get-Date).AddMinutes(5)
    $lastError = $null
    while ((Get-Date) -lt $deadline) {
        try {
            return New-PSSession -VMName $Name -Credential $Credential -ErrorAction Stop
        }
        catch {
            $lastError = $_
            Start-Sleep -Seconds 2
        }
    }
    throw "Timed out creating PowerShell Direct session for $Name. Last error: $lastError"
}

function Invoke-GuestStartupScript {
    param(
        [System.Management.Automation.Runspaces.PSSession]$Session,
        [string]$Mode,
        [string]$ImageRef,
        [string]$ResultName,
        [string]$RootfsPath = ""
    )

    Invoke-Command -Session $Session -ScriptBlock {
        param(
            [string]$GuestWorkDirParam,
            [string]$ModeParam,
            [string]$ImageRefParam,
            [string]$ResultNameParam,
            [string]$RootfsPathParam
        )

        $startupScript = Join-Path $GuestWorkDirParam "discobot-wsl-startup.ps1"
        $resultPath = Join-Path $GuestWorkDirParam $ResultNameParam
        $distroName = "discobot-smoke-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
        $distroNamePath = Join-Path $GuestWorkDirParam "distro-name.txt"
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
        if (Test-Path -LiteralPath $distroNamePath) {
            $distroName = (Get-Content -Raw -LiteralPath $distroNamePath).Trim()
        }
        else {
            Set-Utf8NoBomFile -Path $distroNamePath -Value $distroName
        }

        $scriptArgs = @(
            "-NoProfile",
            "-ExecutionPolicy", "Bypass",
            "-File", $startupScript,
            "-Mode", $ModeParam,
            "-DistroName", $distroName,
            "-InstallDir", (Join-Path $GuestWorkDirParam "distro"),
            "-VarDiskPath", (Join-Path $GuestWorkDirParam "var.vhdx"),
            "-VarDiskSizeGB", "10",
            "-VarDiskLabel", "dbot-smoke-var",
            "-StatePath", (Join-Path $GuestWorkDirParam "runtime-state.json"),
            "-ImageRef", $ImageRefParam,
            "-ResultFile", $resultPath
        )
        if ($RootfsPathParam.Trim() -ne "") {
            $scriptArgs += @("-RootfsArchivePath", $RootfsPathParam)
        }

        $output = & powershell.exe @scriptArgs 2>&1 | ForEach-Object { $_.ToString() }
        $exitCode = $LASTEXITCODE
        if ($null -eq $exitCode) {
            $exitCode = 0
        }

        $result = $null
        if (Test-Path -LiteralPath $resultPath) {
            try {
                $result = Get-Content -Raw -LiteralPath $resultPath | ConvertFrom-Json
            }
            catch {
                $result = $null
            }
        }

        [pscustomobject]@{
            Mode       = $ModeParam
            ExitCode   = [int]$exitCode
            Output     = @($output) -join "`n"
            ResultPath = $resultPath
            Result     = $result
            DistroName = $distroName
        }
    } -ArgumentList $GuestWorkDir, $Mode, $ImageRef, $ResultName, $RootfsPath
}

function Assert-ExitCode {
    param(
        [Parameter(Mandatory = $true)]
        $Result,
        [Parameter(Mandatory = $true)]
        [int]$Expected
    )

    if ($Result.ExitCode -ne $Expected) {
        $message = if ($null -ne $Result.Result -and $Result.Result.message) { $Result.Result.message } else { $Result.Output }
        throw "$($Result.Mode) exited $($Result.ExitCode), expected $Expected. $message"
    }
}

$startupScript = Resolve-RequiredPath -Path $StartupScriptPath -Description "WSL startup script"
$rootfsArchive = Resolve-RequiredPath -Path $RootfsArchivePath -Description "Rootfs import tar"
if ($rootfsArchive.EndsWith(".zst", [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "RootfsArchivePath must be a plain tar import archive, not a .zst file: $rootfsArchive"
}

if ($null -eq $GuestCredential) {
    if ($GuestUsername -ne "" -and $null -ne $GuestPassword) {
        $GuestCredential = [pscredential]::new($GuestUsername, $GuestPassword)
    }
    else {
        $GuestCredential = Get-Credential -Message "Enter local administrator credentials for VM '$VMName'"
    }
}

$vm = Get-VM -Name $VMName -ErrorAction Stop
$checkpointName = "discobot-wsl-startup-test-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
$session = $null
$summary = [ordered]@{
    vmName         = $VMName
    checkpointName = $checkpointName
    startedAt      = (Get-Date).ToUniversalTime().ToString("o")
    phases         = @()
}

New-Item -ItemType Directory -Force -Path $HostOutputDir | Out-Null

try {
    Write-Host "Creating checkpoint '$checkpointName' for VM '$VMName'..."
    Checkpoint-VM -Name $VMName -SnapshotName $checkpointName | Out-Null

    if ($vm.State -ne "Running") {
        Write-Host "Starting VM '$VMName'..."
        Start-VM -Name $VMName | Out-Null
    }
    Wait-VMHeartbeat -Name $VMName

    Write-Host "Opening PowerShell Direct session..."
    $session = New-GuestSession -Name $VMName -Credential $GuestCredential

    Invoke-Command -Session $session -ScriptBlock {
        param([string]$GuestWorkDirParam)
        New-Item -ItemType Directory -Force -Path $GuestWorkDirParam | Out-Null
    } -ArgumentList $GuestWorkDir

    Write-Host "Copying startup script and rootfs tar into guest..."
    Copy-Item -LiteralPath $startupScript -Destination (Join-Path $GuestWorkDir "discobot-wsl-startup.ps1") -ToSession $session -Force
    $guestRootfsPath = Join-Path $GuestWorkDir "rootfs-import.tar"
    Copy-Item -LiteralPath $rootfsArchive -Destination $guestRootfsPath -ToSession $session -Force

    $imageRef = "discobot-smoke:initial"
    $check = Invoke-GuestStartupScript -Session $session -Mode "check" -ImageRef $imageRef -ResultName "check-initial.json"
    $summary.phases += $check
    if ($check.ExitCode -eq 42) {
        $message = if ($null -ne $check.Result -and $check.Result.message) { $check.Result.message } else { $check.Output }
        $summary.status = "wsl_unavailable"
        throw "WSL is not installed or enabled in guest VM '$VMName'. The app should surface this as wsl_not_installed. $message"
    }
    Assert-ExitCode -Result $check -Expected 10
    $actions = @($check.Result.actions)
    if ($actions -notcontains "import-distro") {
        throw "Initial check did not request import-distro. Actions: $($actions -join ', ')"
    }

    $execute = Invoke-GuestStartupScript -Session $session -Mode "execute" -ImageRef $imageRef -ResultName "execute-initial.json" -RootfsPath $guestRootfsPath
    $summary.phases += $execute
    Assert-ExitCode -Result $execute -Expected 0

    $verify = Invoke-GuestStartupScript -Session $session -Mode "check" -ImageRef $imageRef -ResultName "check-verify.json"
    $summary.phases += $verify
    Assert-ExitCode -Result $verify -Expected 0

    if ($TestUpgrade) {
        $upgradeRef = "discobot-smoke:upgrade"
        $upgradeCheck = Invoke-GuestStartupScript -Session $session -Mode "check" -ImageRef $upgradeRef -ResultName "check-upgrade.json"
        $summary.phases += $upgradeCheck
        Assert-ExitCode -Result $upgradeCheck -Expected 10
        $upgradeActions = @($upgradeCheck.Result.actions)
        if ($upgradeActions -notcontains "upgrade-distro") {
            throw "Upgrade check did not request upgrade-distro. Actions: $($upgradeActions -join ', ')"
        }

        $upgradeExecute = Invoke-GuestStartupScript -Session $session -Mode "execute" -ImageRef $upgradeRef -ResultName "execute-upgrade.json" -RootfsPath $guestRootfsPath
        $summary.phases += $upgradeExecute
        Assert-ExitCode -Result $upgradeExecute -Expected 0

        $upgradeVerify = Invoke-GuestStartupScript -Session $session -Mode "check" -ImageRef $upgradeRef -ResultName "check-upgrade-verify.json"
        $summary.phases += $upgradeVerify
        Assert-ExitCode -Result $upgradeVerify -Expected 0
    }

    $summary.completedAt = (Get-Date).ToUniversalTime().ToString("o")
    $summary.status = "passed"
    Write-Host "WSL startup smoke test passed."
}
catch {
    $summary.completedAt = (Get-Date).ToUniversalTime().ToString("o")
    $summary.status = "failed"
    $summary.error = $_.Exception.Message
    throw
}
finally {
    $summaryPath = Join-Path $HostOutputDir ("wsl-startup-hyperv-{0}.json" -f $checkpointName)
    $summaryJson = $summary | ConvertTo-Json -Depth 8
    Set-Utf8NoBomFile -Path $summaryPath -Value $summaryJson
    Write-Host "Wrote summary: $summaryPath"

    if ($null -ne $session) {
        Remove-PSSession $session
    }

    if (-not $NoRestore) {
        Write-Host "Restoring checkpoint '$checkpointName'..."
        Restore-VMSnapshot -VMName $VMName -Name $checkpointName -Confirm:$false
        if (-not $KeepCheckpoint) {
            Remove-VMSnapshot -VMName $VMName -Name $checkpointName -Confirm:$false
        }
    }
    elseif ($KeepCheckpoint) {
        Write-Host "Leaving checkpoint '$checkpointName' in place."
    }
}
