using System.Globalization;
using System.Text;

namespace HcsLinuxVmLauncher;

internal sealed class CliOptions
{
    public Guid VmId { get; set; } = Guid.NewGuid();
    public string Owner { get; set; } = "HcsLinuxVmLauncher";
    public string KernelPath { get; set; } = DefaultWslToolPath("kernel");
    public string? InitrdPath { get; set; } = DefaultWslToolPath("initrd.img");
    public string? RootDiskPath { get; set; }
    public string? DataDiskPath { get; set; }
    public string RootDevice { get; set; } = "/dev/sda1";
    public string? RootFileSystem { get; set; } = "ext4";
    public string? KernelCommandLineOverride { get; set; }
    public string? AppendKernelCommandLine { get; set; }
    public int MemoryMb { get; set; } = HostResources.DefaultMemoryMb();
    public int ProcessorCount { get; set; } = HostResources.DefaultProcessorCount();
    public int VsockPort { get; set; } = 5000;
    public int GvproxyVsockPort { get; set; } = 1024;
    public string GvproxyPath { get; set; } = "gvproxy.exe";
    public string UsernetIp { get; set; } = "192.168.127.2";
    public string UsernetNetmask { get; set; } = "255.255.255.0";
    public string UsernetGateway { get; set; } = "192.168.127.1";
    public string UsernetDns { get; set; } = "192.168.127.1";
    public bool ListenVsock { get; set; }
    public bool EchoVsock { get; set; }
    public string HvSocketSecurityDescriptor { get; set; } = "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;IU)";
    public string? ConsolePipeName { get; set; }
    public List<Plan9ShareOption> Plan9Shares { get; } = new();
    public NetworkMode NetworkMode { get; set; } = NetworkMode.HcnNat;
    public Guid NatNetworkId { get; set; } = Guid.NewGuid();
    public Guid NatEndpointId { get; set; } = Guid.NewGuid();
    public string? NatName { get; set; }
    public string NatSubnet { get; set; } = "172.31.240.0/20";
    public string NatGateway { get; set; } = "172.31.240.1";
    public string? NatVmIp { get; set; }
    public bool NatEnableDhcp { get; set; } = true;
    public bool NatDisableHostPort { get; set; }
    public bool SkipGrantVmAccess { get; set; }
    public bool DryRun { get; set; }
    public bool Help { get; set; }

    public string VmIdString => VmId.ToString("D");
    public string EffectiveNatName => NatName ?? $"hcs-linux-vm-{VmId:N}"[..29];
    public Guid VsockServiceId => HvSocketPorts.PortToServiceId(VsockPort);
    public Guid GvproxyServiceId => HvSocketPorts.PortToServiceId(GvproxyVsockPort);

    public IEnumerable<string> FilesNeedingVmAccess()
    {
        yield return KernelPath;

        if (!string.IsNullOrWhiteSpace(InitrdPath))
        {
            yield return InitrdPath!;
        }

        if (!string.IsNullOrWhiteSpace(RootDiskPath))
        {
            yield return RootDiskPath!;
        }

        if (!string.IsNullOrWhiteSpace(DataDiskPath))
        {
            yield return DataDiskPath!;
        }
    }

    public void Validate(bool validateFiles)
    {
        if (RootDiskPath is null)
        {
            throw new ArgumentException("Missing required --root <root.vhdx> argument.");
        }

        if (DataDiskPath is null)
        {
            throw new ArgumentException("Missing required --data <data.vhdx> argument.");
        }

        if (MemoryMb < 128)
        {
            throw new ArgumentException("--memory-mb must be at least 128.");
        }

        if (ProcessorCount < 1)
        {
            throw new ArgumentException("--processors must be at least 1.");
        }

        if (VsockPort is < 1 or > 0x7fffffff)
        {
            throw new ArgumentException("--vsock-port must be between 1 and 2147483647.");
        }

        if (GvproxyVsockPort is < 1 or > 0x7fffffff)
        {
            throw new ArgumentException("--gvproxy-vsock-port must be between 1 and 2147483647.");
        }

        if (NetworkMode == NetworkMode.UserVsock && ListenVsock && VsockPort == GvproxyVsockPort)
        {
            throw new ArgumentException("--listen-vsock cannot use the same port as --gvproxy-vsock-port.");
        }

        if (NetworkMode == NetworkMode.UserVsock)
        {
            _ = Ipv4Address.Parse(UsernetIp);
            _ = Ipv4Address.Parse(UsernetNetmask);
            _ = Ipv4Address.Parse(UsernetGateway);
            _ = Ipv4Address.Parse(UsernetDns);
        }

        if (NetworkMode == NetworkMode.HcnNat)
        {
            Ipv4Cidr.Parse(NatSubnet);
            _ = Ipv4Address.Parse(NatGateway);
            if (!string.IsNullOrWhiteSpace(NatVmIp))
            {
                _ = Ipv4Address.Parse(NatVmIp!);
            }
        }

        if (!validateFiles)
        {
            return;
        }

        foreach (var file in FilesNeedingVmAccess())
        {
            if (!File.Exists(file))
            {
                throw new FileNotFoundException($"Required VM asset was not found: {file}", file);
            }
        }

        foreach (var share in Plan9Shares)
        {
            if (!Directory.Exists(share.HostPath))
            {
                throw new DirectoryNotFoundException($"Plan9 share directory was not found: {share.HostPath}");
            }
        }

        if (NetworkMode == NetworkMode.UserVsock)
        {
            GvproxyPath = ResolveExecutablePath(GvproxyPath);
        }
    }

    private static string ResolveExecutablePath(string executable)
    {
        if (string.IsNullOrWhiteSpace(executable))
        {
            throw new ArgumentException("--gvproxy requires a non-empty executable path.");
        }

        if (Path.IsPathFullyQualified(executable) || executable.Contains(Path.DirectorySeparatorChar) || executable.Contains(Path.AltDirectorySeparatorChar))
        {
            if (File.Exists(executable))
            {
                return executable;
            }

            throw new FileNotFoundException($"gvproxy executable was not found: {executable}", executable);
        }

        var path = Environment.GetEnvironmentVariable("PATH") ?? string.Empty;
        var extensions = Path.HasExtension(executable)
            ? new[] { string.Empty }
            : (Environment.GetEnvironmentVariable("PATHEXT") ?? ".COM;.EXE;.BAT;.CMD").Split(';', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        foreach (var directory in path.Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries))
        {
            foreach (var extension in extensions)
            {
                var candidate = Path.Combine(directory, executable + extension);
                if (File.Exists(candidate))
                {
                    return candidate;
                }
            }
        }

        throw new FileNotFoundException($"gvproxy executable was not found on PATH: {executable}", executable);
    }

    private static string DefaultWslToolPath(string fileName)
    {
        var programFiles = Environment.GetEnvironmentVariable("ProgramFiles");
        if (string.IsNullOrWhiteSpace(programFiles))
        {
            programFiles = @"C:\Program Files";
        }

        return $@"{programFiles}\WSL\tools\{fileName}";
    }
}

internal enum NetworkMode
{
    None,
    HcnNat,
    UserVsock,
}

internal static class CliParser
{
    public static CliOptions Parse(string[] args)
    {
        var options = new CliOptions();

        for (var i = 0; i < args.Length; i++)
        {
            var arg = args[i];
            switch (arg)
            {
                case "-h":
                case "--help":
                    options.Help = true;
                    break;
                case "--dry-run":
                    options.DryRun = true;
                    break;
                case "--id":
                    options.VmId = Guid.Parse(RequireValue(args, ref i, arg));
                    break;
                case "--owner":
                    options.Owner = RequireValue(args, ref i, arg);
                    break;
                case "--kernel":
                    options.KernelPath = RequireValue(args, ref i, arg);
                    break;
                case "--initrd":
                    options.InitrdPath = RequireValue(args, ref i, arg);
                    break;
                case "--no-initrd":
                    options.InitrdPath = null;
                    break;
                case "--root":
                    options.RootDiskPath = RequireValue(args, ref i, arg);
                    break;
                case "--data":
                    options.DataDiskPath = RequireValue(args, ref i, arg);
                    break;
                case "--root-device":
                    options.RootDevice = RequireValue(args, ref i, arg);
                    break;
                case "--root-fstype":
                    options.RootFileSystem = RequireValue(args, ref i, arg);
                    break;
                case "--no-root-fstype":
                    options.RootFileSystem = null;
                    break;
                case "--kernel-cmdline":
                    options.KernelCommandLineOverride = RequireValue(args, ref i, arg);
                    break;
                case "--append-kernel-cmdline":
                    options.AppendKernelCommandLine = Append(options.AppendKernelCommandLine, RequireValue(args, ref i, arg));
                    break;
                case "--memory-mb":
                    options.MemoryMb = int.Parse(RequireValue(args, ref i, arg), CultureInfo.InvariantCulture);
                    break;
                case "--ram-gb":
                case "--memory-gb":
                    options.MemoryMb = HostResources.MemoryGbToMb(decimal.Parse(RequireValue(args, ref i, arg), CultureInfo.InvariantCulture));
                    break;
                case "--processors":
                    options.ProcessorCount = int.Parse(RequireValue(args, ref i, arg), CultureInfo.InvariantCulture);
                    break;
                case "--vsock-port":
                    options.VsockPort = int.Parse(RequireValue(args, ref i, arg));
                    break;
                case "--gvproxy":
                    options.GvproxyPath = RequireValue(args, ref i, arg);
                    break;
                case "--gvproxy-vsock-port":
                    options.GvproxyVsockPort = int.Parse(RequireValue(args, ref i, arg));
                    break;
                case "--usernet-ip":
                    options.UsernetIp = RequireValue(args, ref i, arg);
                    break;
                case "--usernet-netmask":
                    options.UsernetNetmask = RequireValue(args, ref i, arg);
                    break;
                case "--usernet-gateway":
                    options.UsernetGateway = RequireValue(args, ref i, arg);
                    break;
                case "--usernet-dns":
                    options.UsernetDns = RequireValue(args, ref i, arg);
                    break;
                case "--listen-vsock":
                    options.ListenVsock = true;
                    break;
                case "--echo-vsock":
                    options.EchoVsock = true;
                    break;
                case "--hvsocket-sddl":
                    options.HvSocketSecurityDescriptor = RequireValue(args, ref i, arg);
                    break;
                case "--console-pipe":
                    options.ConsolePipeName = RequireValue(args, ref i, arg);
                    break;
                case "--share":
                    options.Plan9Shares.Add(ParsePlan9Share(RequireValue(args, ref i, arg), readOnly: true));
                    break;
                case "--share-rw":
                    options.Plan9Shares.Add(ParsePlan9Share(RequireValue(args, ref i, arg), readOnly: false));
                    break;
                case "--network":
                    options.NetworkMode = ParseNetworkMode(RequireValue(args, ref i, arg));
                    break;
                case "--nat-network-id":
                    options.NatNetworkId = Guid.Parse(RequireValue(args, ref i, arg));
                    break;
                case "--nat-endpoint-id":
                    options.NatEndpointId = Guid.Parse(RequireValue(args, ref i, arg));
                    break;
                case "--nat-name":
                    options.NatName = RequireValue(args, ref i, arg);
                    break;
                case "--nat-subnet":
                    options.NatSubnet = RequireValue(args, ref i, arg);
                    break;
                case "--nat-gateway":
                    options.NatGateway = RequireValue(args, ref i, arg);
                    break;
                case "--nat-vm-ip":
                    options.NatVmIp = RequireValue(args, ref i, arg);
                    break;
                case "--nat-disable-dhcp":
                    options.NatEnableDhcp = false;
                    break;
                case "--nat-disable-host-port":
                    options.NatDisableHostPort = true;
                    break;
                case "--skip-grant-vm-access":
                    options.SkipGrantVmAccess = true;
                    break;
                default:
                    throw new ArgumentException($"Unknown argument: {arg}");
            }
        }

        if (string.IsNullOrWhiteSpace(options.ConsolePipeName))
        {
            options.ConsolePipeName = $@"\\.\pipe\hcs-linux-vm-{options.VmId:N}-hvc0";
        }

        return options;
    }

    public static string HelpText()
    {
        var exe = "dotnet run --";
        var builder = new StringBuilder();
        builder.AppendLine("Launch a Linux utility VM through the Windows Host Compute System API.");
        builder.AppendLine();
        builder.AppendLine("Required:");
        builder.AppendLine($"  {exe} --root C:\\vm\\rootfs.vhdx --data C:\\vm\\data.vhdx [options]");
        builder.AppendLine();
        builder.AppendLine("Important options:");
        builder.AppendLine("  --kernel <path>                 WSL2 kernel path. Default: %ProgramFiles%\\WSL\\tools\\kernel");
        builder.AppendLine("  --initrd <path>                 WSL2 initrd path. Default: %ProgramFiles%\\WSL\\tools\\initrd.img");
        builder.AppendLine("  --no-initrd                     Do not pass an initrd to LinuxKernelDirect.");
        builder.AppendLine("  --root-device <dev>             Kernel root= value. Default: /dev/sda1");
        builder.AppendLine("  --root-fstype <type>            Kernel rootfstype= value. Default: ext4");
        builder.AppendLine("  --kernel-cmdline <text>         Replace the generated kernel command line.");
        builder.AppendLine("  --append-kernel-cmdline <text>  Append extra kernel command-line options.");
        builder.AppendLine("  --memory-mb <n>                 VM memory in MiB. Default: 50% of host physical memory");
        builder.AppendLine("  --ram-gb <n>                    VM memory in GiB. Alias: --memory-gb");
        builder.AppendLine("  --processors <n>                vCPU count. Default: host logical processor count");
        builder.AppendLine("  --vsock-port <n>                Linux AF_VSOCK port exposed via Hyper-V socket. Default: 5000");
        builder.AppendLine("  --gvproxy <path>                gvproxy executable for --network user-vsock. Default: gvproxy.exe");
        builder.AppendLine("  --gvproxy-vsock-port <n>        VSOCK port for gvproxy user networking. Default: 1024");
        builder.AppendLine("  --usernet-ip <ip>               Static guest IP for --network user-vsock. Default: 192.168.127.2");
        builder.AppendLine("  --usernet-netmask <ip>          Static guest netmask for --network user-vsock. Default: 255.255.255.0");
        builder.AppendLine("  --usernet-gateway <ip>          Static gateway for --network user-vsock. Default: 192.168.127.1");
        builder.AppendLine("  --usernet-dns <ip>              Static DNS server for --network user-vsock. Default: 192.168.127.1");
        builder.AppendLine("  --listen-vsock                  Open a host HVSOCK listener and print received bytes.");
        builder.AppendLine("  --echo-vsock                    Echo bytes received by --listen-vsock.");
        builder.AppendLine("  --hvsocket-sddl <sddl>          Override HVSOCK bind/connect security descriptor.");
        builder.AppendLine("  --share <name=host-dir>         Add a read-only Plan9/9P host directory share.");
        builder.AppendLine("  --share-rw <name=host-dir>      Add a read/write Plan9/9P host directory share.");
        builder.AppendLine("  --network hcn-nat|none|user-vsock");
        builder.AppendLine("                                   hcn-nat attaches HCN ICS/NAT, none attaches no NIC,");
        builder.AppendLine("                                   user-vsock runs gvproxy over Hyper-V sockets. Default: hcn-nat");
        builder.AppendLine("  --nat-subnet <cidr>             HCN NAT subnet. Default: 172.31.240.0/20");
        builder.AppendLine("  --nat-gateway <ip>              HCN NAT gateway. Default: 172.31.240.1");
        builder.AppendLine("  --nat-vm-ip <ip>                Request a specific VM endpoint IPv4 address.");
        builder.AppendLine("  --nat-disable-dhcp              Do not request HNS DHCP support on the NAT network.");
        builder.AppendLine("  --nat-disable-host-port         Request HNS DisableHostPort on the NAT network.");
        builder.AppendLine("  --skip-grant-vm-access          Skip HcsGrantVmAccess/HcsRevokeVmAccess around VM assets.");
        builder.AppendLine("  --dry-run                       Print generated HCS JSON and exit.");
        builder.AppendLine();
        builder.AppendLine("The root VHDX is attached read-only at SCSI LUN 0; the data VHDX is attached read/write at LUN 1.");
        return builder.ToString();
    }

    private static string RequireValue(string[] args, ref int index, string option)
    {
        if (index + 1 >= args.Length)
        {
            throw new ArgumentException($"{option} requires a value.");
        }

        index++;
        return args[index];
    }

    private static Plan9ShareOption ParsePlan9Share(string value, bool readOnly)
    {
        var separator = value.IndexOf('=');
        if (separator <= 0 || separator == value.Length - 1)
        {
            throw new ArgumentException("Plan9 share must use <name=host-dir> syntax.");
        }

        var name = value[..separator];
        var hostPath = value[(separator + 1)..];
        if (string.IsNullOrWhiteSpace(name) || name.IndexOfAny(['/', '\\', ':', ' ', '\t', '\r', '\n']) >= 0)
        {
            throw new ArgumentException($"Invalid Plan9 share name '{name}'. Use a simple mount tag without whitespace, slashes, or colons.");
        }

        if (string.IsNullOrWhiteSpace(hostPath) || !Path.IsPathFullyQualified(hostPath))
        {
            throw new ArgumentException($"Plan9 share path must be fully qualified: {hostPath}");
        }

        return new Plan9ShareOption(name, hostPath, readOnly);
    }

    private static NetworkMode ParseNetworkMode(string value) => value.ToLowerInvariant() switch
    {
        "none" => NetworkMode.None,
        "hcn-nat" => NetworkMode.HcnNat,
        "nat" => NetworkMode.HcnNat,
        "user-vsock" => NetworkMode.UserVsock,
        "gvproxy" => NetworkMode.UserVsock,
        "slirp" => NetworkMode.UserVsock,
        _ => throw new ArgumentException($"Unsupported --network value '{value}'. Use 'hcn-nat', 'none', or 'user-vsock'."),
    };

    private static string Append(string? existing, string value)
    {
        if (string.IsNullOrWhiteSpace(existing))
        {
            return value;
        }

        return existing + " " + value;
    }
}
