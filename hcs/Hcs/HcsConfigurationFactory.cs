using System.Text.Json;
using System.Text.Json.Serialization;

namespace HcsLinuxVmLauncher;

internal static class HcsConfigurationFactory
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    public static string Build(CliOptions options)
    {
        var kernelCommandLine = BuildKernelCommandLine(options);

        var hvSocketConfig = new Dictionary<string, object?>
        {
            ["DefaultBindSecurityDescriptor"] = options.HvSocketSecurityDescriptor,
            ["DefaultConnectSecurityDescriptor"] = options.HvSocketSecurityDescriptor,
        };

        var hvSocketServiceTable = BuildHvSocketServiceTable(options);
        if (hvSocketServiceTable.Count > 0)
        {
            hvSocketConfig["ServiceTable"] = hvSocketServiceTable;
        }

        var attachments = new Dictionary<string, object>
        {
            ["0"] = Attachment(options.RootDiskPath!, readOnly: true),
            ["1"] = Attachment(options.DataDiskPath!, readOnly: false),
        };

        var comPorts = new Dictionary<string, object>();
        var virtioSerialPorts = new Dictionary<string, object>();
        if (!string.IsNullOrWhiteSpace(options.ConsolePipeName))
        {
            virtioSerialPorts["0"] = new
            {
                NamedPipe = options.ConsolePipeName,
                Name = "hvc0",
                ConsoleSupport = true,
            };
        }

        var devices = new Dictionary<string, object?>
        {
            ["ComPorts"] = comPorts,
            ["Scsi"] = new Dictionary<string, object>
            {
                ["0"] = new { Attachments = attachments },
            },
            ["HvSocket"] = new { HvSocketConfig = hvSocketConfig },
            ["Plan9"] = new { },
            ["Battery"] = new { },
        };

        if (virtioSerialPorts.Count > 0)
        {
            devices["VirtioSerial"] = new { Ports = virtioSerialPorts };
        }

        var document = new
        {
            Owner = options.Owner,
            SchemaVersion = new { Major = 2, Minor = 3 },
            ShouldTerminateOnLastHandleClosed = true,
            VirtualMachine = new
            {
                StopOnReset = true,
                Chipset = new
                {
                    UseUtc = true,
                    LinuxKernelDirect = new
                    {
                        KernelFilePath = options.KernelPath,
                        InitRdPath = options.InitrdPath,
                        KernelCmdLine = kernelCommandLine,
                    },
                },
                ComputeTopology = new
                {
                    Memory = new
                    {
                        SizeInMB = HostResources.AlignMemoryMb(options.MemoryMb),
                        AllowOvercommit = true,
                        EnableDeferredCommit = true,
                        EnableColdDiscardHint = true,
                        HighMmioBaseInMB = 49152,
                        HighMmioGapInMB = 16384,
                        HostingProcessNameSuffix = "HcsLinuxVmLauncher",
                    },
                    Processor = new
                    {
                        Count = options.ProcessorCount,
                    },
                },
                Devices = devices,
                DebugOptions = new { },
            },
        };

        return JsonSerializer.Serialize(document, JsonOptions);
    }

    public static string BuildKernelCommandLine(CliOptions options)
    {
        if (!string.IsNullOrWhiteSpace(options.KernelCommandLineOverride))
        {
            var commandLine = options.KernelCommandLineOverride!;
            if (options.NetworkMode == NetworkMode.UserVsock)
            {
                commandLine = Append(commandLine, BuildDiscobotKernelOption(options));
            }

            return Append(commandLine, options.AppendKernelCommandLine);
        }

        var parts = new List<string>();
        if (!string.IsNullOrWhiteSpace(options.InitrdPath))
        {
            parts.Add(@"initrd=\initrd.img");
        }

        parts.Add($"root={options.RootDevice}");
        if (!string.IsNullOrWhiteSpace(options.RootFileSystem))
        {
            parts.Add($"rootfstype={options.RootFileSystem}");
        }

        parts.Add("ro");
        parts.Add("rootwait");
        parts.Add("panic=1");
        parts.Add($"nr_cpus={options.ProcessorCount}");
        parts.Add("hv_utils.timesync_implicit=1");
        parts.Add("console=hvc0");
        parts.Add("earlyprintk=serial");
        parts.Add("pty.legacy_count=0");

        if (options.NetworkMode == NetworkMode.UserVsock)
        {
            parts.Add(BuildDiscobotKernelOption(options));
        }

        return Append(string.Join(' ', parts), options.AppendKernelCommandLine);
    }

    private static string BuildDiscobotKernelOption(CliOptions options)
    {
        return string.Join(',',
            $"discobot=ip={options.UsernetIp}",
            $"netmask={options.UsernetNetmask}",
            $"gateway={options.UsernetGateway}",
            $"dns={options.UsernetDns}");
    }

    private static Dictionary<string, object> BuildHvSocketServiceTable(CliOptions options)
    {
        var services = new Dictionary<string, object>(StringComparer.OrdinalIgnoreCase);
        if (options.NetworkMode == NetworkMode.UserVsock)
        {
            AddHvSocketService(services, options.GvproxyServiceId, options.HvSocketSecurityDescriptor);
        }

        if (options.ListenVsock)
        {
            AddHvSocketService(services, options.VsockServiceId, options.HvSocketSecurityDescriptor);
        }

        return services;
    }

    private static void AddHvSocketService(Dictionary<string, object> services, Guid serviceId, string securityDescriptor)
    {
        services[serviceId.ToString("D")] = new
        {
            BindSecurityDescriptor = securityDescriptor,
            ConnectSecurityDescriptor = securityDescriptor,
        };
    }

    private static object Attachment(string path, bool readOnly) => new
    {
        Type = "VirtualDisk",
        Path = path,
        ReadOnly = readOnly,
        SupportCompressedVolumes = true,
        AlwaysAllowSparseFiles = true,
        SupportEncryptedFiles = true,
    };

    private static string Append(string commandLine, string? append)
    {
        if (string.IsNullOrWhiteSpace(append))
        {
            return commandLine;
        }

        return commandLine + " " + append;
    }
}
