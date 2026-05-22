[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSAvoidUsingConvertToSecureStringWithPlainText", "", Justification = "Validation script creates a disposable local VM account from a generated or operator-provided password.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSAvoidUsingWriteHost", "", Justification = "Validation script is intentionally interactive and operator-facing.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseApprovedVerbs", "", Justification = "Internal helper names describe setup phases rather than exported commands.")]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute("PSUseShouldProcessForStateChangingFunctions", "", Justification = "Top-level script supports ShouldProcess for external state changes.")]
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
    [string]$VMName,
    [string]$IsoPath,
    [string]$VMRoot = (Join-Path $env:PUBLIC "Documents\Hyper-V"),
    [int]$VhdSizeGB = 128,
    [int]$MemoryStartupGB = 8,
    [int]$ProcessorCount = 4,
    [string]$SwitchName,
    [string]$NatSwitchName = "NestedNAT",
    [string]$NatSubnet = "192.168.250.0/24",
    [string]$NatGatewayIp = "192.168.250.1",
    [string]$EditionName = "Windows 11 Pro",
    [int]$ImageIndex,
    [string]$ProductKey,
    [string]$ComputerName,
    [string]$AdminUsername = "admin",
    [securestring]$AdminPassword,
    [string]$TimeZone = "UTC",
    [switch]$ManualInstall,
    [switch]$WaitForInstall,
    [int]$InstallTimeoutMinutes = 90,
    [int]$ValidationRetrySeconds = 15,
    [bool]$OpenConsole = $true
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Cyan
}

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

function Assert-Administrator {
    $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
    if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Run this script from an elevated PowerShell session."
    }
}

function Resolve-Windows11Iso {
    param([string]$RequestedPath)

    if ($RequestedPath) {
        $resolved = Resolve-Path -Path $RequestedPath -ErrorAction Stop
        return $resolved.Path
    }

    $downloads = Join-Path $HOME "Downloads"
    if (-not (Test-Path -Path $downloads)) {
        throw "Could not find your Downloads folder at '$downloads'. Pass -IsoPath explicitly."
    }

    $preferred = @(Get-ChildItem -Path $downloads -Filter "*.iso" -File |
            Where-Object { $_.Name -match "(?i)win(dows)?[ _-]?11" } |
            Sort-Object LastWriteTime -Descending)
    if ($preferred.Count -gt 0) {
        return $preferred[0].FullName
    }

    $fallback = @(Get-ChildItem -Path $downloads -Filter "*.iso" -File |
            Sort-Object LastWriteTime -Descending)
    if ($fallback.Count -gt 0) {
        return $fallback[0].FullName
    }

    throw "No ISO file was found in '$downloads'. Pass -IsoPath explicitly."
}

function Ensure-HyperVInstalled {
    $feature = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All
    if ($feature.State -eq "Enabled") {
        return
    }

    Write-Step "Enabling Hyper-V features"
    $null = Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -All -NoRestart

    throw "Hyper-V was enabled, but Windows must reboot before the VM can be created. Reboot, then run the script again."
}

function Assert-HardwareSupport {
    $cpu = Get-CimInstance -ClassName Win32_Processor | Select-Object -First 1
    if (-not $cpu.VirtualizationFirmwareEnabled) {
        throw "Hardware virtualization is disabled in firmware/BIOS. Enable AMD-V or SVM mode, then rerun the script."
    }

    # Some AMD and nested-host environments do not report SLAT reliably through WMI,
    # even when Hyper-V is already available. Warn instead of hard-failing here.
    if ($null -ne $cpu.SecondLevelAddressTranslationExtensions -and -not $cpu.SecondLevelAddressTranslationExtensions) {
        Write-Warning "The host did not positively report SLAT support through WMI. If Hyper-V creation fails later, verify BIOS virtualization settings and host Hyper-V support."
    }
}

function Resolve-VMSwitchName {
    param(
        [string]$RequestedSwitchName,
        [string]$RequestedNatSwitchName,
        [string]$RequestedNatSubnet,
        [string]$RequestedNatGatewayIp
    )

    if ($RequestedSwitchName) {
        $switch = Get-VMSwitch -Name $RequestedSwitchName -ErrorAction SilentlyContinue
        if (-not $switch) {
            throw "Hyper-V switch '$RequestedSwitchName' does not exist."
        }

        return $switch.Name
    }

    $defaultSwitch = Get-VMSwitch -Name "Default Switch" -ErrorAction SilentlyContinue
    if ($defaultSwitch) {
        return $defaultSwitch.Name
    }

    $natSwitch = Get-VMSwitch -Name $RequestedNatSwitchName -ErrorAction SilentlyContinue
    if (-not $natSwitch) {
        Write-Step "Creating NAT switch '$RequestedNatSwitchName'"
        $natSwitch = New-VMSwitch -Name $RequestedNatSwitchName -SwitchType Internal
    }

    $adapterName = "vEthernet ($($natSwitch.Name))"
    $adapter = Get-NetAdapter -Name $adapterName -ErrorAction SilentlyContinue
    if (-not $adapter) {
        throw "Could not find host adapter '$adapterName' for NAT switch configuration."
    }

    $prefixLength = [int](($RequestedNatSubnet -split "/")[-1])
    $existingAddress = Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.IPAddress -eq $RequestedNatGatewayIp }
    if (-not $existingAddress) {
        Write-Step "Assigning $RequestedNatGatewayIp to '$adapterName'"
        New-NetIPAddress -InterfaceIndex $adapter.ifIndex -IPAddress $RequestedNatGatewayIp -PrefixLength $prefixLength | Out-Null
    }

    $natName = "$RequestedNatSwitchName-NAT"
    $existingNat = Get-NetNat -Name $natName -ErrorAction SilentlyContinue
    if (-not $existingNat) {
        Write-Step "Creating NAT network '$natName' for $RequestedNatSubnet"
        New-NetNat -Name $natName -InternalIPInterfaceAddressPrefix $RequestedNatSubnet | Out-Null
    }

    return $natSwitch.Name
}

function ConvertTo-PlainText {
    param([securestring]$SecureValue)

    if (-not $SecureValue) {
        return $null
    }

    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureValue)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    }
}

function New-RandomPassword {
    param([int]$Length = 24)

    $chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@$%*-_".ToCharArray()
    -join (1..$Length | ForEach-Object { $chars[(Get-Random -Minimum 0 -Maximum $chars.Length)] })
}

function Escape-Xml {
    param([AllowNull()][string]$Value)
    return [Security.SecurityElement]::Escape($Value)
}

function Resolve-VMName {
    param([string]$RequestedVMName)

    if (-not [string]::IsNullOrWhiteSpace($RequestedVMName)) {
        return $RequestedVMName.Trim()
    }

    return "Win11-{0}-{1}" -f (Get-Date -Format 'yyyyMMdd-HHmmss'), (Get-Random -Minimum 1000 -Maximum 9999)
}

function Resolve-ComputerName {
    param(
        [string]$RequestedComputerName,
        [string]$DefaultName
    )

    $name = if ($RequestedComputerName) { $RequestedComputerName } else { $DefaultName }
    $name = $name -replace '[^A-Za-z0-9-]', ''
    $name = $name.Trim('-')
    if (-not $name) {
        $name = 'WIN11VM'
    }

    if ($name.Length -gt 15) {
        $name = $name.Substring(0, 15)
    }

    return $name
}

function Resolve-AdminPasswordInfo {
    param([securestring]$RequestedPassword)

    if ($RequestedPassword) {
        return [pscustomobject]@{
            Secure      = $RequestedPassword
            Plain       = ConvertTo-PlainText -SecureValue $RequestedPassword
            UsedDefault = $false
        }
    }

    $plain = 'adminpass'
    return [pscustomobject]@{
        Secure      = ConvertTo-SecureString -String $plain -AsPlainText -Force
        Plain       = $plain
        UsedDefault = $true
    }
}

function New-LocalCredential {
    param(
        [string]$UserName,
        [securestring]$Password
    )

    return [pscredential]::new(".\$UserName", $Password)
}

function Get-DefaultProductKey {
    param([string]$ResolvedEditionName)

    $edition = $ResolvedEditionName.ToLowerInvariant()
    if ($edition -match 'windows 11 pro n') {
        return '2B87N-8KFHP-DKV6R-Y2C8J-PKCKT'
    }

    if ($edition -match 'windows 11 pro') {
        return 'VK7JG-NPHTM-C97JM-9MPGT-3V66T'
    }

    if ($edition -match 'windows 11 home n') {
        return '3KHY7-WNT83-DGQKR-F7HPR-844BM'
    }

    if ($edition -match 'windows 11 home') {
        return 'TX9XD-98N7V-6WMQ6-BX7FG-H8Q99'
    }

    return $null
}

function Get-WindowsImageSelection {
    param(
        [string]$ResolvedIsoPath,
        [string]$RequestedEditionName,
        [int]$RequestedImageIndex
    )

    $mounted = $false
    $diskImage = $null

    try {
        Write-Step "Inspecting Windows images in '$ResolvedIsoPath'"
        $diskImage = Mount-DiskImage -ImagePath $ResolvedIsoPath -PassThru
        $mounted = $true

        $volume = $diskImage | Get-Volume | Where-Object { $_.DriveLetter } | Select-Object -First 1
        if (-not $volume) {
            throw "Could not determine a mounted drive letter for '$ResolvedIsoPath'."
        }

        $mountRoot = "$($volume.DriveLetter):\"
        $installImagePath = Join-Path $mountRoot 'sources\install.wim'
        if (-not (Test-Path -Path $installImagePath)) {
            $installImagePath = Join-Path $mountRoot 'sources\install.esd'
        }

        if (-not (Test-Path -Path $installImagePath)) {
            throw "Could not find sources\install.wim or sources\install.esd inside the Windows ISO."
        }

        $images = @(Get-WindowsImage -ImagePath $installImagePath)
        if ($images.Count -eq 0) {
            throw "No installable Windows images were found in '$installImagePath'."
        }

        if ($RequestedImageIndex -gt 0) {
            $match = @($images | Where-Object { $_.ImageIndex -eq $RequestedImageIndex })
            if ($match.Count -eq 0) {
                throw "Image index $RequestedImageIndex was not found in the Windows ISO."
            }

            return [pscustomobject]@{
                ImageIndex = $match[0].ImageIndex
                ImageName  = $match[0].ImageName
            }
        }

        $exact = @($images | Where-Object { $_.ImageName -eq $RequestedEditionName })
        if ($exact.Count -eq 1) {
            return [pscustomobject]@{
                ImageIndex = $exact[0].ImageIndex
                ImageName  = $exact[0].ImageName
            }
        }

        $fuzzy = @($images | Where-Object { $_.ImageName -like "*$RequestedEditionName*" })
        if ($fuzzy.Count -eq 1) {
            return [pscustomobject]@{
                ImageIndex = $fuzzy[0].ImageIndex
                ImageName  = $fuzzy[0].ImageName
            }
        }

        if ($images.Count -eq 1) {
            return [pscustomobject]@{
                ImageIndex = $images[0].ImageIndex
                ImageName  = $images[0].ImageName
            }
        }

        $availableImages = $images | ForEach-Object { "[$($_.ImageIndex)] $($_.ImageName)" }
        throw "The ISO contains multiple Windows images. Pass -EditionName or -ImageIndex. Available images: $($availableImages -join '; ')"
    }
    finally {
        if ($mounted -and $diskImage) {
            Dismount-DiskImage -ImagePath $ResolvedIsoPath | Out-Null
        }
    }
}

function Ensure-IsoWriterType {
    if ('ISOFile' -as [type]) {
        return
    }

    Add-Type -TypeDefinition @"
using System;
using System.IO;
using System.Runtime.InteropServices;

public static class ISOFile
{
    public static void Create(string path, object stream, int blockSize, int totalBlocks)
    {
        using (var isoStream = File.Create(path))
        {
            var ptr = Marshal.GetIUnknownForObject(stream);
            try
            {
                var imageStream = (IStream)Marshal.GetTypedObjectForIUnknown(ptr, typeof(IStream));
                var buffer = new byte[blockSize];
                for (int block = 0; block < totalBlocks; block++)
                {
                    imageStream.Read(buffer, blockSize, IntPtr.Zero);
                    isoStream.Write(buffer, 0, blockSize);
                }
                isoStream.Flush();
                Marshal.ReleaseComObject(imageStream);
            }
            finally
            {
                Marshal.Release(ptr);
            }
        }
    }

    [ComImport, Guid("0000000c-0000-0000-C000-000000000046"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IStream
    {
        void Read([MarshalAs(UnmanagedType.LPArray, SizeParamIndex = 1)] byte[] pv, int cb, IntPtr pcbRead);
        void Write([MarshalAs(UnmanagedType.LPArray, SizeParamIndex = 1)] byte[] pv, int cb, IntPtr pcbWritten);
        void Seek(long dlibMove, int dwOrigin, IntPtr plibNewPosition);
        void SetSize(long libNewSize);
        void CopyTo(IStream pstm, long cb, IntPtr pcbRead, IntPtr pcbWritten);
        void Commit(int grfCommitFlags);
        void Revert();
        void LockRegion(long libOffset, long cb, int dwLockType);
        void UnlockRegion(long libOffset, long cb, int dwLockType);
        void Stat(IntPtr pstatstg, int grfStatFlag);
        void Clone(out IStream ppstm);
    }
}
"@
}

function New-IsoFromFolder {
    param(
        [string]$SourcePath,
        [string]$DestinationPath,
        [string]$VolumeName,
        [string]$EfiBootImagePath
    )

    Ensure-IsoWriterType

    $image = New-Object -ComObject IMAPI2FS.MsftFileSystemImage
    $sourceBytes = @(
        Get-ChildItem -LiteralPath $SourcePath -Recurse -Force -File |
            Measure-Object -Property Length -Sum
    )[0].Sum
    if ($null -eq $sourceBytes) {
        $sourceBytes = 0
    }

    # IMAPI defaults to a small disc size unless we raise the limit. Size the image
    # from the staged content and add some headroom for filesystem metadata.
    $image.FreeMediaBlocks = [int][Math]::Ceiling(($sourceBytes + 64MB) / 2048)
    $image.VolumeName = $VolumeName
    $image.FileSystemsToCreate = 4
    $image.Root.AddTree($SourcePath, $false)

    $bootStream = $null
    $bootOptions = $null
    $result = $null
    if ($EfiBootImagePath) {
        if (-not (Test-Path -Path $EfiBootImagePath)) {
            throw "EFI boot image '$EfiBootImagePath' was not found."
        }

        $bootStream = New-Object -ComObject ADODB.Stream
        $bootStream.Type = 1
        $bootStream.Open()
        $bootStream.LoadFromFile($EfiBootImagePath)

        $bootOptions = New-Object -ComObject IMAPI2FS.BootOptions
        $bootOptions.AssignBootImage($bootStream)
        $bootOptions.PlatformId = 0xEF
        $bootOptions.Emulation = 0
        $image.BootImageOptions = $bootOptions
    }

    try {
        $result = $image.CreateResultImage()
        [ISOFile]::Create($DestinationPath, $result.ImageStream, $result.BlockSize, $result.TotalBlocks)
    }
    finally {
        if ($bootStream) {
            $bootStream.Close()
        }

        foreach ($comObject in @($result, $bootOptions, $bootStream, $image)) {
            if ($comObject -and [Runtime.InteropServices.Marshal]::IsComObject($comObject)) {
                [void][Runtime.InteropServices.Marshal]::ReleaseComObject($comObject)
            }
        }

        [GC]::Collect()
        [GC]::WaitForPendingFinalizers()
    }
}

function New-NoPromptWindowsSetupIso {
    param(
        [string]$SourceIsoPath,
        [string]$DestinationIsoPath,
        [string]$StagePath
    )

    $diskImage = $null
    $mounted = $false

    try {
        Write-Step "Creating a no-prompt Windows setup ISO"
        $diskImage = Mount-DiskImage -ImagePath $SourceIsoPath -PassThru
        $mounted = $true

        $volume = $diskImage | Get-Volume | Where-Object { $_.DriveLetter } | Select-Object -First 1
        if (-not $volume) {
            throw "Could not determine a mounted drive letter for '$SourceIsoPath'."
        }

        $mountRoot = "$($volume.DriveLetter):\"
        $volumeName = if ([string]::IsNullOrWhiteSpace($volume.FileSystemLabel)) { 'WIN11_SETUP' } else { $volume.FileSystemLabel }
        $efiBootImagePath = Join-Path $mountRoot 'efi\microsoft\boot\efisys_noprompt.bin'
        if (-not (Test-Path -Path $efiBootImagePath)) {
            throw "The Windows ISO does not contain efi\microsoft\boot\efisys_noprompt.bin, so the DVD boot prompt cannot be suppressed automatically."
        }

        Write-Step "Staging Windows setup files for the no-prompt ISO"
        if (Test-Path -Path $StagePath) {
            Remove-Item -Path $StagePath -Recurse -Force
        }

        New-Item -Path $StagePath -ItemType Directory -Force | Out-Null
        Get-ChildItem -LiteralPath $mountRoot -Force | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $StagePath -Recurse -Force
        }

        New-IsoFromFolder -SourcePath $StagePath -DestinationPath $DestinationIsoPath -VolumeName $volumeName -EfiBootImagePath $efiBootImagePath
        return $DestinationIsoPath
    }
    finally {
        if ($mounted -and $diskImage) {
            Dismount-DiskImage -ImagePath $SourceIsoPath | Out-Null
        }

        if (Test-Path -Path $StagePath) {
            foreach ($attempt in 1..3) {
                try {
                    Remove-Item -Path $StagePath -Recurse -Force -ErrorAction Stop
                    break
                }
                catch {
                    if ($attempt -eq 3) {
                        Write-Warning "Could not remove temporary setup-media folder '$StagePath'. Remove it manually later if needed."
                    }
                    else {
                        Start-Sleep -Seconds 2
                    }
                }
            }
        }
    }
}

function New-AutounattendContent {
    param(
        [pscustomobject]$ImageSelection,
        [string]$ResolvedProductKey,
        [string]$ResolvedComputerName,
        [string]$ResolvedAdminUsername,
        [SecureString]$ResolvedAdminPassword,
        [string]$ResolvedTimeZone
    )

    $productKeyBlock = ""
    if ($ResolvedProductKey) {
        $productKeyBlock = @"
        <ProductKey>
          <Key>$(Escape-Xml $ResolvedProductKey)</Key>
          <WillShowUI>Never</WillShowUI>
        </ProductKey>
"@
    }

    return @"
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend">
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <SetupUILanguage>
        <UILanguage>en-US</UILanguage>
      </SetupUILanguage>
      <InputLocale>en-US</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
    <component name="Microsoft-Windows-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
      <DiskConfiguration>
        <Disk wcm:action="add">
          <DiskID>0</DiskID>
          <WillWipeDisk>true</WillWipeDisk>
          <CreatePartitions>
            <CreatePartition wcm:action="add">
              <Order>1</Order>
              <Type>EFI</Type>
              <Size>100</Size>
            </CreatePartition>
            <CreatePartition wcm:action="add">
              <Order>2</Order>
              <Type>MSR</Type>
              <Size>16</Size>
            </CreatePartition>
            <CreatePartition wcm:action="add">
              <Order>3</Order>
              <Type>Primary</Type>
              <Extend>true</Extend>
            </CreatePartition>
          </CreatePartitions>
          <ModifyPartitions>
            <ModifyPartition wcm:action="add">
              <Order>1</Order>
              <PartitionID>1</PartitionID>
              <Format>FAT32</Format>
              <Label>System</Label>
            </ModifyPartition>
            <ModifyPartition wcm:action="add">
              <Order>2</Order>
              <PartitionID>3</PartitionID>
              <Format>NTFS</Format>
              <Label>Windows</Label>
              <Letter>C</Letter>
            </ModifyPartition>
          </ModifyPartitions>
        </Disk>
        <WillShowUI>OnError</WillShowUI>
      </DiskConfiguration>
      <ImageInstall>
        <OSImage>
          <InstallFrom>
            <MetaData wcm:action="add">
              <Key>/IMAGE/INDEX</Key>
              <Value>$(Escape-Xml ([string]$ImageSelection.ImageIndex))</Value>
            </MetaData>
          </InstallFrom>
          <InstallTo>
            <DiskID>0</DiskID>
            <PartitionID>3</PartitionID>
          </InstallTo>
          <WillShowUI>OnError</WillShowUI>
        </OSImage>
      </ImageInstall>
      <UserData>
        <AcceptEula>true</AcceptEula>
$productKeyBlock        <FullName>VM Admin</FullName>
        <Organization>Local Lab</Organization>
      </UserData>
    </component>
  </settings>
  <settings pass="specialize">
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <ComputerName>$(Escape-Xml $ResolvedComputerName)</ComputerName>
      <TimeZone>$(Escape-Xml $ResolvedTimeZone)</TimeZone>
      <RegisteredOwner>VM Admin</RegisteredOwner>
      <RegisteredOrganization>Local Lab</RegisteredOrganization>
    </component>
  </settings>
  <settings pass="oobeSystem">
    <component name="Microsoft-Windows-International-Core" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <InputLocale>en-US</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
      <AutoLogon>
        <Password>
          <Value>userpass</Value>
          <PlainText>true</PlainText>
        </Password>
        <Domain>.</Domain>
        <Enabled>true</Enabled>
        <LogonCount>999</LogonCount>
        <Username>user</Username>
      </AutoLogon>
      <OOBE>
        <HideEULAPage>true</HideEULAPage>
        <HideOnlineAccountScreens>true</HideOnlineAccountScreens>
        <HideWirelessSetupInOOBE>true</HideWirelessSetupInOOBE>
        <ProtectYourPC>3</ProtectYourPC>
      </OOBE>
      <UserAccounts>
        <LocalAccounts>
          <LocalAccount wcm:action="add">
            <Name>$(Escape-Xml $ResolvedAdminUsername)</Name>
            <Group>Administrators</Group>
            <DisplayName>$(Escape-Xml $ResolvedAdminUsername)</DisplayName>
            <Password>
              <Value>$(Escape-Xml $ResolvedAdminPassword)</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
          <LocalAccount wcm:action="add">
            <Name>user</Name>
            <Group>Users</Group>
            <DisplayName>user</DisplayName>
            <Password>
              <Value>userpass</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
        </LocalAccounts>
      </UserAccounts>
    </component>
  </settings>
</unattend>
"@
}

function Wait-ForGuestValidation {
    param(
        [string]$TargetVMName,
        [pscredential]$Credential,
        [string]$ExpectedEditionName,
        [int]$TimeoutMinutes,
        [int]$RetrySeconds
    )

    $deadline = (Get-Date).AddMinutes($TimeoutMinutes)
    $attempt = 0
    $lastError = $null

    while ((Get-Date) -lt $deadline) {
        $attempt++

        $vm = Get-VM -Name $TargetVMName -ErrorAction SilentlyContinue
        if (-not $vm) {
            throw "VM '$TargetVMName' no longer exists."
        }

        if ($vm.State -ne 'Running') {
            throw "VM '$TargetVMName' is no longer running. Current state: $($vm.State)"
        }

        try {
            $guestInfo = Invoke-Command -VMName $TargetVMName -Credential $Credential -ErrorAction Stop -ScriptBlock {
                $os = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'
                $cpu = Get-CimInstance -ClassName Win32_Processor | Select-Object -First 1
                $computerSystem = Get-CimInstance -ClassName Win32_ComputerSystem

                [pscustomobject]@{
                    ComputerName                            = $env:COMPUTERNAME
                    ProductName                             = $os.ProductName
                    DisplayVersion                          = $os.DisplayVersion
                    CurrentBuild                            = $os.CurrentBuild
                    BuildLabEx                              = $os.BuildLabEx
                    Manufacturer                            = $computerSystem.Manufacturer
                    Model                                   = $computerSystem.Model
                    VirtualizationFirmwareEnabled           = $cpu.VirtualizationFirmwareEnabled
                    VMMonitorModeExtensions                 = $cpu.VMMonitorModeExtensions
                    SecondLevelAddressTranslationExtensions = $cpu.SecondLevelAddressTranslationExtensions
                }
            }

            return [pscustomobject]@{
                ComputerName                            = $guestInfo.ComputerName
                ProductName                             = $guestInfo.ProductName
                DisplayVersion                          = $guestInfo.DisplayVersion
                CurrentBuild                            = $guestInfo.CurrentBuild
                BuildLabEx                              = $guestInfo.BuildLabEx
                Manufacturer                            = $guestInfo.Manufacturer
                Model                                   = $guestInfo.Model
                VirtualizationFirmwareEnabled           = $guestInfo.VirtualizationFirmwareEnabled
                VMMonitorModeExtensions                 = $guestInfo.VMMonitorModeExtensions
                SecondLevelAddressTranslationExtensions = $guestInfo.SecondLevelAddressTranslationExtensions
                ExpectedEdition                         = $ExpectedEditionName
                ExpectedEditionMatched                  = if ($ExpectedEditionName) { $guestInfo.ProductName -like "*$ExpectedEditionName*" } else { $null }
            }
        }
        catch {
            $lastError = $_
            $heartbeat = Get-VMIntegrationService -VMName $TargetVMName -Name 'Heartbeat' -ErrorAction SilentlyContinue
            $heartbeatStatus = if ($heartbeat) { $heartbeat.PrimaryStatusDescription } else { 'Unavailable' }
            Write-Step "Waiting for Windows Setup to finish (attempt $attempt, heartbeat: $heartbeatStatus)"
            Start-Sleep -Seconds $RetrySeconds
        }
    }

    $lastMessage = if ($lastError) { $lastError.Exception.Message } else { 'No guest response received.' }
    throw "Timed out waiting for unattended setup in '$TargetVMName' after $TimeoutMinutes minutes. Last error: $lastMessage"
}

function Enable-GuestRemoteAccess {
    param(
        [string]$TargetVMName,
        [pscredential]$Credential,
        [string]$StandardUserName
    )

    return Invoke-Command -VMName $TargetVMName -Credential $Credential -ErrorAction Stop -ArgumentList $StandardUserName -ScriptBlock {
        param([string]$StandardUserName)

        $resolvedUser = ".\$StandardUserName"
        $groupsUpdated = @()

        foreach ($groupName in @('Remote Management Users', 'Remote Desktop Users')) {
            $group = Get-LocalGroup -Name $groupName -ErrorAction SilentlyContinue
            if (-not $group) {
                continue
            }

            $member = Get-LocalGroupMember -Group $groupName -Member $resolvedUser -ErrorAction SilentlyContinue
            if (-not $member) {
                Add-LocalGroupMember -Group $groupName -Member $resolvedUser
            }

            $groupsUpdated += $groupName
        }

        Enable-PSRemoting -Force -SkipNetworkProfileCheck | Out-Null
        Set-Service -Name WinRM -StartupType Automatic
        Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name 'fDenyTSConnections' -Type DWord -Value 0
        Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -Name 'UserAuthentication' -Type DWord -Value 1
        Enable-NetFirewallRule -DisplayGroup 'Windows Remote Management' -ErrorAction SilentlyContinue | Out-Null
        Enable-NetFirewallRule -DisplayGroup 'Remote Desktop' -ErrorAction SilentlyContinue | Out-Null
        Set-Service -Name TermService -StartupType Manual
        Start-Service -Name TermService -ErrorAction SilentlyContinue

        [pscustomobject]@{
            StandardUserName     = $StandardUserName
            RemoteGroups         = $groupsUpdated
            WinRMStartupType     = (Get-Service -Name WinRM).StartType
            TermServiceStatus    = (Get-Service -Name TermService).Status
            RemoteDesktopEnabled = ((Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name 'fDenyTSConnections').fDenyTSConnections -eq 0)
        }
    }
}

function Test-GuestRemoteSession {
    param(
        [string]$TargetVMName,
        [pscredential]$Credential,
        [int]$TimeoutSeconds = 120
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = $null

    while ((Get-Date) -lt $deadline) {
        try {
            return Invoke-Command -VMName $TargetVMName -Credential $Credential -ErrorAction Stop -ScriptBlock {
                [pscustomobject]@{
                    WhoAmI       = whoami
                    ComputerName = $env:COMPUTERNAME
                }
            }
        }
        catch {
            $lastError = $_
            Start-Sleep -Seconds 5
        }
    }

    $lastMessage = if ($lastError) { $lastError.Exception.Message } else { 'No guest response received.' }
    throw "Timed out testing remote access for '$($Credential.UserName)' in '$TargetVMName'. Last error: $lastMessage"
}

function Wait-ForGuestRdpReady {
    param(
        [string]$TargetVMName,
        [pscredential]$Credential,
        [int]$TimeoutSeconds = 180
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = $null

    while ((Get-Date) -lt $deadline) {
        try {
            $guestInfo = Invoke-Command -VMName $TargetVMName -Credential $Credential -ErrorAction Stop -ScriptBlock {
                $addresses = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
                    Where-Object {
                        $_.IPAddress -and
                        $_.IPAddress -notlike '169.254.*' -and
                        $_.IPAddress -ne '127.0.0.1'
                    } |
                    Sort-Object SkipAsSource, InterfaceMetric

                [pscustomobject]@{
                    IPv4Addresses     = @($addresses | Select-Object -ExpandProperty IPAddress)
                    TermServiceStatus = (Get-Service -Name TermService).Status
                }
            }

            foreach ($ipAddress in @($guestInfo.IPv4Addresses)) {
                if (-not $ipAddress) {
                    continue
                }

                $rdpReady = Test-NetConnection -ComputerName $ipAddress -Port 3389 -InformationLevel Quiet -WarningAction SilentlyContinue
                if ($rdpReady) {
                    return [pscustomobject]@{
                        IPAddress         = $ipAddress
                        Port              = 3389
                        TermServiceStatus = $guestInfo.TermServiceStatus
                    }
                }
            }

            $lastError = "No host-reachable RDP listener found yet. Guest IPs: $(@($guestInfo.IPv4Addresses) -join ', ')"
            Start-Sleep -Seconds 5
        }
        catch {
            $lastError = $_.Exception.Message
            Start-Sleep -Seconds 5
        }
    }

    throw "Timed out waiting for RDP readiness in '$TargetVMName'. Last error: $lastError"
}

Assert-Administrator
Assert-HardwareSupport
Ensure-HyperVInstalled

$resolvedIsoPath = Resolve-Windows11Iso -RequestedPath $IsoPath
$generatedVMName = [string]::IsNullOrWhiteSpace($VMName)
$VMName = Resolve-VMName -RequestedVMName $VMName
$vmPath = Join-Path $VMRoot $VMName
$vhdPath = Join-Path $vmPath "$VMName.vhdx"
$memoryStartupBytes = $MemoryStartupGB * 1GB
$vhdSizeBytes = $VhdSizeGB * 1GB
$resolvedComputerName = Resolve-ComputerName -RequestedComputerName $ComputerName -DefaultName $VMName
$standardUsername = 'user'
$standardUserPassword = 'userpass'
$standardUserPasswordSecure = ConvertTo-SecureString -String $standardUserPassword -AsPlainText -Force

if ($WaitForInstall -and $ManualInstall) {
    throw "-WaitForInstall requires unattended setup. Remove -ManualInstall or run validation manually later."
}

if (Get-VM -Name $VMName -ErrorAction SilentlyContinue) {
    throw "A VM named '$VMName' already exists. Choose a different -VMName or remove the existing VM first."
}

$imageSelection = $null
$resolvedProductKey = $null
$passwordInfo = $null
$installIsoPath = $resolvedIsoPath
$noPromptInstallIsoPath = $null
$autounattendXmlPath = $null
$autounattendIsoPath = $null
$vmCreated = $false
$validationResult = $null
$remoteAccessResult = $null
$standardUserSessionResult = $null
$rdpReadyResult = $null

if (-not $ManualInstall) {
    $imageSelection = Get-WindowsImageSelection -ResolvedIsoPath $resolvedIsoPath -RequestedEditionName $EditionName -RequestedImageIndex $ImageIndex
    $passwordInfo = Resolve-AdminPasswordInfo -RequestedPassword $AdminPassword
    $resolvedProductKey = if ($ProductKey) { $ProductKey } else { Get-DefaultProductKey -ResolvedEditionName $imageSelection.ImageName }

    if (-not $resolvedProductKey) {
        Write-Warning "No known generic setup key was found for '$($imageSelection.ImageName)'. Windows Setup may still prompt for a product key unless you pass -ProductKey."
    }
}

Write-Step "Using ISO '$resolvedIsoPath'"
if ($generatedVMName) {
    Write-Step "Generated VM name '$VMName'"
}
if ($imageSelection) {
    Write-Step "Selected Windows image [$($imageSelection.ImageIndex)] $($imageSelection.ImageName)"
}

if ($PSCmdlet.ShouldProcess($VMName, "Create Windows 11 Hyper-V VM")) {
    Write-Step "Creating VM folder '$vmPath'"
    New-Item -Path $vmPath -ItemType Directory -Force | Out-Null

    $noPromptInstallIsoPath = Join-Path $vmPath 'Windows11-Setup-NoPrompt.iso'
    $noPromptSetupStagePath = Join-Path $vmPath 'setup-media'
    $installIsoPath = New-NoPromptWindowsSetupIso -SourceIsoPath $resolvedIsoPath -DestinationIsoPath $noPromptInstallIsoPath -StagePath $noPromptSetupStagePath

    if (-not $ManualInstall) {
        Write-Step "Generating unattended installation media"
        $autounattendXmlPath = Join-Path $vmPath 'Autounattend.xml'
        $autounattendIsoPath = Join-Path $vmPath 'Autounattend.iso'
        $autounattendStagePath = Join-Path $vmPath 'autounattend-media'
        New-Item -Path $autounattendStagePath -ItemType Directory -Force | Out-Null

        $autounattendContent = New-AutounattendContent -ImageSelection $imageSelection -ResolvedProductKey $resolvedProductKey -ResolvedComputerName $resolvedComputerName -ResolvedAdminUsername $AdminUsername -ResolvedAdminPassword $passwordInfo.Plain -ResolvedTimeZone $TimeZone
        Set-Utf8NoBomFile -Path $autounattendXmlPath -Value $autounattendContent
        Set-Utf8NoBomFile -Path (Join-Path $autounattendStagePath 'Autounattend.xml') -Value $autounattendContent
        New-IsoFromFolder -SourcePath $autounattendStagePath -DestinationPath $autounattendIsoPath -VolumeName 'AUTOUNATTEND'
    }

    $switchToUse = Resolve-VMSwitchName -RequestedSwitchName $SwitchName -RequestedNatSwitchName $NatSwitchName -RequestedNatSubnet $NatSubnet -RequestedNatGatewayIp $NatGatewayIp
    Write-Step "Using virtual switch '$switchToUse'"

    Write-Step "Creating Generation 2 VM '$VMName'"
    New-VM -Name $VMName `
        -Generation 2 `
        -Path $vmPath `
        -MemoryStartupBytes $memoryStartupBytes `
        -NewVHDPath $vhdPath `
        -NewVHDSizeBytes $vhdSizeBytes `
        -SwitchName $switchToUse | Out-Null

    Write-Step "Configuring memory, CPU, and checkpoints"
    Set-VMMemory -VMName $VMName -DynamicMemoryEnabled $false
    Set-VMProcessor -VMName $VMName -Count $ProcessorCount -ExposeVirtualizationExtensions $true
    Set-VM -Name $VMName -AutomaticCheckpointsEnabled $false

    Write-Step "Attaching Windows 11 setup ISO"
    $setupDvdDrive = Get-VMDvdDrive -VMName $VMName | Select-Object -First 1
    if ($setupDvdDrive) {
        Set-VMDvdDrive -VMName $VMName -ControllerNumber $setupDvdDrive.ControllerNumber -ControllerLocation $setupDvdDrive.ControllerLocation -Path $installIsoPath | Out-Null
        $setupDvdDrive = Get-VMDvdDrive -VMName $VMName | Where-Object { $_.Path -eq $installIsoPath } | Select-Object -First 1
    }
    else {
        $setupDvdDrive = Add-VMDvdDrive -VMName $VMName -Path $installIsoPath
    }

    if (-not $ManualInstall -and $autounattendIsoPath) {
        Write-Step "Attaching Autounattend ISO"
        Add-VMDvdDrive -VMName $VMName -Path $autounattendIsoPath | Out-Null
    }

    Write-Step "Enabling Secure Boot and virtual TPM"
    Set-VMFirmware -VMName $VMName -EnableSecureBoot On -SecureBootTemplate 'MicrosoftWindows' -FirstBootDevice $setupDvdDrive
    Set-VMKeyProtector -VMName $VMName -NewLocalKeyProtector
    Enable-VMTPM -VMName $VMName

    Write-Step "Allowing nested virtualization and nested guest networking"
    Get-VMNetworkAdapter -VMName $VMName | Set-VMNetworkAdapter -MacAddressSpoofing On

    Write-Step "Starting VM '$VMName'"
    Start-VM -Name $VMName | Out-Null
    $vmCreated = $true

    if ($OpenConsole) {
        $vmConnect = Get-Command vmconnect.exe -ErrorAction SilentlyContinue
        if ($vmConnect) {
            Write-Step "Opening VM console"
            Start-Process -FilePath $vmConnect.Source -ArgumentList 'localhost', $VMName
        }
        else {
            Write-Warning "vmconnect.exe was not found on PATH. Open the VM manually from Hyper-V Manager."
        }
    }

    if ($WaitForInstall) {
        Write-Step "Waiting for unattended setup to finish and PowerShell Direct to become available"
        $guestCredential = New-LocalCredential -UserName $AdminUsername -Password $passwordInfo.Secure
        $validationResult = Wait-ForGuestValidation -TargetVMName $VMName -Credential $guestCredential -ExpectedEditionName $imageSelection.ImageName -TimeoutMinutes $InstallTimeoutMinutes -RetrySeconds $ValidationRetrySeconds
        Write-Step "Granting remote-session access to '$standardUsername'"
        $remoteAccessResult = Enable-GuestRemoteAccess -TargetVMName $VMName -Credential $guestCredential -StandardUserName $standardUsername
        Write-Step "Testing guest remote sign-in for '$standardUsername'"
        $standardUserCredential = New-LocalCredential -UserName $standardUsername -Password $standardUserPasswordSecure
        $standardUserSessionResult = Test-GuestRemoteSession -TargetVMName $VMName -Credential $standardUserCredential
        Write-Step "Waiting for Remote Desktop to become reachable for '$standardUsername'"
        $rdpReadyResult = Wait-ForGuestRdpReady -TargetVMName $VMName -Credential $guestCredential
    }
}

Write-Host ""

if (-not $vmCreated) {
    Write-Host "No VM was created. If you used -WhatIf, rerun without it to apply the changes." -ForegroundColor Yellow
    return
}

Write-Host "VM '$VMName' is ready and booting from the Windows 11 ISO." -ForegroundColor Green
if ($noPromptInstallIsoPath) {
    Write-Host "No-prompt setup ISO: '$noPromptInstallIsoPath'." -ForegroundColor Green
}
Write-Host "Nested virtualization is enabled via Hyper-V processor extensions exposure." -ForegroundColor Green

if ($ManualInstall) {
    Write-Host "Manual install mode is enabled, so complete Windows Setup from the VM console." -ForegroundColor Yellow
}
else {
    Write-Host "Unattended setup media was created at '$autounattendIsoPath'." -ForegroundColor Green
    Write-Host "The guest local administrator account is '$AdminUsername'." -ForegroundColor Green
    Write-Host "The guest standard user account is '$standardUsername'." -ForegroundColor Green
    Write-Host "Auto-logon is configured for '$standardUsername'." -ForegroundColor Green
    if ($passwordInfo.UsedDefault) {
        Write-Host "Default password: $($passwordInfo.Plain)" -ForegroundColor Yellow
    }
    else {
        Write-Host "The supplied administrator password was written into Autounattend.xml for Windows Setup." -ForegroundColor Yellow
    }
    Write-Host "Standard user password: $standardUserPassword" -ForegroundColor Yellow

    if ($validationResult) {
        Write-Host "Guest validation succeeded over PowerShell Direct." -ForegroundColor Green
        Write-Host "Installed OS: $($validationResult.ProductName) (build $($validationResult.CurrentBuild))" -ForegroundColor Green
        Write-Host "Guest computer name: $($validationResult.ComputerName)" -ForegroundColor Green
        if ($null -ne $validationResult.ExpectedEditionMatched -and -not $validationResult.ExpectedEditionMatched) {
            Write-Warning "The installed guest edition '$($validationResult.ProductName)' did not match the requested image '$($validationResult.ExpectedEdition)'."
        }

        if ($validationResult.VMMonitorModeExtensions -or $validationResult.VirtualizationFirmwareEnabled) {
            Write-Host "The guest reports virtualization support, which is a good sign for nested virtualization." -ForegroundColor Green
        }
        else {
            Write-Warning "The guest did not clearly report virtualization support through WMI. Verify nested virtualization manually inside the guest if needed."
        }
    }

    if ($remoteAccessResult) {
        Write-Host "The guest standard user was granted remote-session access via: $($remoteAccessResult.RemoteGroups -join ', ')." -ForegroundColor Green
        if ($remoteAccessResult.RemoteDesktopEnabled) {
            Write-Host "Remote Desktop was enabled for the guest." -ForegroundColor Green
        }
        if ($remoteAccessResult.TermServiceStatus) {
            Write-Host "Remote Desktop service status: $($remoteAccessResult.TermServiceStatus)." -ForegroundColor Green
        }
    }

    if ($standardUserSessionResult) {
        Write-Host "Verified guest remote sign-in as '$standardUsername' ($($standardUserSessionResult.WhoAmI))." -ForegroundColor Green
    }

    if ($rdpReadyResult) {
        Write-Host "Verified Remote Desktop readiness for '$standardUsername' at $($rdpReadyResult.IPAddress):$($rdpReadyResult.Port)." -ForegroundColor Green
        Write-Host "Connect with: mstsc /v:$($rdpReadyResult.IPAddress)" -ForegroundColor Green
    }
}

Write-Host "If you plan to run Hyper-V inside the guest, keep dynamic memory disabled as configured here." -ForegroundColor Green
