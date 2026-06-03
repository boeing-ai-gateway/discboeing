using System.Net;
using System.Net.Sockets;
using System.Runtime.InteropServices;

namespace HcsLinuxVmLauncher;

internal static class ProgramEntry
{
    public static async Task<int> RunAsync(string[] args)
    {
        CliOptions options;
        try
        {
            options = CliParser.Parse(args);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine(ex.Message);
            Console.Error.WriteLine();
            Console.Error.Write(CliParser.HelpText());
            return 2;
        }

        if (options.Help)
        {
            Console.Write(CliParser.HelpText());
            return 0;
        }

        try
        {
            options.Validate(validateFiles: !options.DryRun);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine(ex.Message);
            return 2;
        }

        var hcsJson = HcsConfigurationFactory.Build(options);
        if (options.DryRun)
        {
            PrintDryRun(options, hcsJson);
            return 0;
        }

        if (!OperatingSystem.IsWindows())
        {
            Console.Error.WriteLine("This launcher uses Windows HCS/HCN APIs and must be run on Windows.");
            return 1;
        }

        using var cancellation = new CancellationTokenSource();
        using var signalRegistration = RegisterTerminationSignals(cancellation);
        VmAccessGrant? accessGrant = null;
        HcnNatAttachment? nat = null;
        HcsComputeSystem? computeSystem = null;
        HvSocketServer? hvSocketServer = null;
        GvProxyProcess? gvproxy = null;
        HvSocketTcpProxy? gvproxyBridge = null;
        int? gvproxyTcpPort = null;
        var plan9Shares = new List<Plan9ShareAttachment>();

        try
        {
            if (!options.SkipGrantVmAccess)
            {
                Console.WriteLine("Granting the VM access to kernel, initrd, and VHDX files...");
                accessGrant = VmAccessGrant.Grant(options);
            }

            if (options.NetworkMode == NetworkMode.UserVsock)
            {
                gvproxyTcpPort = AllocateLoopbackTcpPort();
                Console.WriteLine($"Starting gvproxy user-mode networking on {GvProxyProcess.TcpListenUri(gvproxyTcpPort.Value)}...");
                gvproxy = GvProxyProcess.Start(options, gvproxyTcpPort.Value);
            }

            Console.WriteLine($"Creating HCS VM {options.VmId:D}...");
            computeSystem = HcsComputeSystem.Create(options.VmId, hcsJson);

            if (options.NetworkMode == NetworkMode.UserVsock)
            {
                gvproxyBridge = HvSocketTcpProxy.Start(options.VmId, options.GvproxyServiceId, IPAddress.Loopback.ToString(), gvproxyTcpPort!.Value, cancellation.Token);
                Console.WriteLine($"Bridging Hyper-V socket service {options.GvproxyServiceId:D} (VSOCK port {options.GvproxyVsockPort}) to gvproxy.");
            }

            Console.WriteLine("Starting VM...");
            computeSystem.Start();

            foreach (var share in options.Plan9Shares)
            {
                Console.WriteLine($"Adding Plan9 share '{share.Name}' for {share.HostPath} ({(share.ReadOnly ? "read-only" : "read/write")})...");
                plan9Shares.Add(Plan9ShareAttachment.Add(computeSystem, share));
                PrintPlan9ShareSummary(share);
            }

            if (options.NetworkMode == NetworkMode.HcnNat)
            {
                Console.WriteLine($"Creating HCN NAT network '{options.EffectiveNatName}' and endpoint...");
                nat = HcnNatAttachment.Create(options);

                Console.WriteLine($"Attaching HCN endpoint {options.NatEndpointId:D} to the VM...");
                nat.Attach(computeSystem);
                PrintEndpointSummary(nat.Properties);
            }

            if (options.ListenVsock)
            {
                hvSocketServer = HvSocketServer.Start(options.VmId, options.VsockPort, options.EchoVsock, cancellation.Token);
                Console.WriteLine($"Listening on Hyper-V socket service {options.VsockServiceId:D} (VSOCK port {options.VsockPort}).");
            }

            Console.WriteLine();
            Console.WriteLine("VM is running. Press Ctrl+C or send process termination to stop it.");
            await WaitUntilCancelledAsync(cancellation.Token).ConfigureAwait(false);
            return 0;
        }
        catch (OperationCanceledException)
        {
            return 0;
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine(ex.Message);
            return 1;
        }
        finally
        {
            for (var i = plan9Shares.Count - 1; i >= 0; i--)
            {
                plan9Shares[i].Dispose();
            }

            hvSocketServer?.Dispose();

            if (computeSystem is not null)
            {
                Console.WriteLine("Terminating VM...");
                try
                {
                    computeSystem.Terminate();
                }
                catch (Exception ex)
                {
                    Console.Error.WriteLine($"Warning: VM termination failed: {ex.Message}");
                }

                computeSystem.Dispose();
            }

            nat?.Dispose();
            gvproxyBridge?.Dispose();
            gvproxy?.Dispose();
            accessGrant?.Dispose();
        }
    }

    private static void PrintDryRun(CliOptions options, string hcsJson)
    {
        Console.WriteLine("# HCS compute system JSON");
        Console.WriteLine(hcsJson);
        Console.WriteLine();
        Console.WriteLine($"# Hyper-V socket service GUID for VSOCK port {options.VsockPort}");
        Console.WriteLine(options.VsockServiceId.ToString("D"));

        if (options.NetworkMode == NetworkMode.HcnNat)
        {
            Console.WriteLine();
            Console.WriteLine("# HCN network JSON");
            Console.WriteLine(HcnConfigurationFactory.BuildNetwork(options));
            Console.WriteLine();
            Console.WriteLine("# HCN endpoint JSON");
            Console.WriteLine(HcnConfigurationFactory.BuildEndpoint(options));
        }
        else if (options.NetworkMode == NetworkMode.UserVsock)
        {
            Console.WriteLine();
            Console.WriteLine("# gvproxy user-mode networking");
            PrintGvproxySummary(options);
        }

        foreach (var share in options.Plan9Shares)
        {
            Console.WriteLine();
            Console.WriteLine($"# HCS Plan9 share add JSON for '{share.Name}'");
            Console.WriteLine(Plan9ShareConfigurationFactory.BuildAdd(share));
            Console.WriteLine();
            PrintPlan9ShareSummary(share);
        }
    }

    private static void PrintPlan9ShareSummary(Plan9ShareOption share)
    {
        var guestPath = $"/mnt/{share.Name}";
        Console.WriteLine($"Plan9 share '{share.Name}' ({(share.ReadOnly ? "read-only" : "read/write")}) host path: {share.HostPath}");
        Console.WriteLine("Guest mount hint:");
        Console.WriteLine($"  mkdir -p {guestPath}");
        Console.WriteLine($"  {share.MountCommand(guestPath)}");
    }

    private static void PrintGvproxySummary(CliOptions options)
    {
        Console.WriteLine("No HCN network or VM NIC will be attached.");
        Console.WriteLine($"gvproxy command: {GvProxyProcess.CommandLine(options)}");
        Console.WriteLine($"Launcher bridge: {GvProxyProcess.HvSocketListenUri(options)} -> tcp://127.0.0.1:<auto>");
        Console.WriteLine($"Hyper-V socket service GUID: {options.GvproxyServiceId:D}");
        Console.WriteLine($"Guest static config: discobot=ip={options.UsernetIp},netmask={options.UsernetNetmask},gateway={options.UsernetGateway},dns={options.UsernetDns}");
        Console.WriteLine("Windows host prerequisite, run once from elevated PowerShell if the service key does not exist:");
        Console.WriteLine($"  $service = New-Item -Force -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices' -Name '{options.GvproxyServiceId:D}'");
        Console.WriteLine("  $service.SetValue('ElementName', 'gvisor-tap-vsock')");
        Console.WriteLine("Guest prerequisite: run gvforwarder in the VM with hv_sock and tun support available.");
    }

    private static int AllocateLoopbackTcpPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        try
        {
            return ((IPEndPoint)listener.LocalEndpoint).Port;
        }
        finally
        {
            listener.Stop();
        }
    }

    private static IDisposable RegisterTerminationSignals(CancellationTokenSource cancellation)
    {
        var registrations = new List<IDisposable>();

        ConsoleCancelEventHandler consoleHandler = (_, eventArgs) =>
        {
            eventArgs.Cancel = true;
            cancellation.Cancel();
        };
        Console.CancelKeyPress += consoleHandler;
        registrations.Add(new DelegateDisposable(() => Console.CancelKeyPress -= consoleHandler));

        TryRegisterSignal(PosixSignal.SIGTERM, cancellation, registrations);
        TryRegisterSignal(PosixSignal.SIGINT, cancellation, registrations);

        return new CompositeDisposable(registrations);
    }

    private static void TryRegisterSignal(
        PosixSignal signal,
        CancellationTokenSource cancellation,
        ICollection<IDisposable> registrations)
    {
        try
        {
            registrations.Add(PosixSignalRegistration.Create(signal, context =>
            {
                context.Cancel = true;
                cancellation.Cancel();
            }));
        }
        catch (PlatformNotSupportedException)
        {
        }
    }

    private static async Task WaitUntilCancelledAsync(CancellationToken cancellationToken)
    {
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken).ConfigureAwait(false);
    }

    private static void PrintEndpointSummary(HcnEndpointProperties properties)
    {
        Console.WriteLine($"Endpoint ID: {properties.Id:D}");
        Console.WriteLine($"Endpoint MAC: {properties.MacAddress}");
        if (!string.IsNullOrWhiteSpace(properties.IpAddress))
        {
            Console.WriteLine($"Endpoint IPv4: {properties.IpAddress}");
        }

        if (!string.IsNullOrWhiteSpace(properties.GatewayAddress))
        {
            Console.WriteLine($"Endpoint gateway: {properties.GatewayAddress}");
        }
    }

    private sealed class DelegateDisposable : IDisposable
    {
        private readonly Action _dispose;
        private bool _disposed;

        public DelegateDisposable(Action dispose)
        {
            _dispose = dispose;
        }

        public void Dispose()
        {
            if (_disposed)
            {
                return;
            }

            _disposed = true;
            _dispose();
        }
    }

    private sealed class CompositeDisposable : IDisposable
    {
        private readonly IReadOnlyList<IDisposable> _items;

        public CompositeDisposable(IReadOnlyList<IDisposable> items)
        {
            _items = items;
        }

        public void Dispose()
        {
            foreach (var item in _items)
            {
                item.Dispose();
            }
        }
    }
}
