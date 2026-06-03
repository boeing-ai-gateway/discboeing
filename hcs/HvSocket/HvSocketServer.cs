using System.Buffers;
using System.ComponentModel;
using System.Runtime.InteropServices;

namespace HcsLinuxVmLauncher;

internal sealed class HvSocketServer : IDisposable
{
    private const int AfHyperV = 34;
    private const int SockStream = 1;
    private const int HvProtocolRaw = 1;
    private const int SocketError = -1;
    private const int WsaWouldBlock = 10035;
    private static readonly nint InvalidSocket = new(-1);
    private static readonly TimeSpan AcceptPollInterval = TimeSpan.FromMilliseconds(100);

    private readonly nint _listenSocket;
    private readonly CancellationTokenSource _stop;
    private readonly Task _acceptLoop;
    private bool _disposed;

    private HvSocketServer(nint listenSocket, bool echo, CancellationToken cancellationToken)
    {
        _listenSocket = listenSocket;
        _stop = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        _acceptLoop = Task.Run(() => AcceptLoop(echo, _stop.Token));
    }

    public static HvSocketServer Start(Guid vmId, int port, bool echo, CancellationToken cancellationToken)
    {
        if (!OperatingSystem.IsWindows())
        {
            throw new PlatformNotSupportedException("Hyper-V sockets are only available on Windows.");
        }

        WindowsSockets.Startup();
        var socket = WindowsSockets.WSASocketW(AfHyperV, SockStream, HvProtocolRaw, nint.Zero, 0, 0);
        if (socket == InvalidSocket)
        {
            WindowsSockets.Cleanup();
            throw WindowsSockets.Exception("WSASocket(AF_HYPERV)");
        }

        try
        {
            var address = SockAddrHv.Create(vmId, HvSocketPorts.PortToServiceId(port));
            if (WindowsSockets.bind(socket, ref address, Marshal.SizeOf<SockAddrHv>()) == SocketError)
            {
                throw WindowsSockets.Exception("bind(AF_HYPERV)");
            }

            if (WindowsSockets.listen(socket, 8) == SocketError)
            {
                throw WindowsSockets.Exception("listen(AF_HYPERV)");
            }

            WindowsSockets.SetNonBlocking(socket, enabled: true);
            return new HvSocketServer(socket, echo, cancellationToken);
        }
        catch
        {
            _ = WindowsSockets.closesocket(socket);
            WindowsSockets.Cleanup();
            throw;
        }
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;
        _stop.Cancel();

        var closeTask = CloseSocketAndCleanupAsync(_listenSocket);
        if (!closeTask.Wait(TimeSpan.FromSeconds(2)))
        {
            Console.Error.WriteLine("Warning: AF_HYPERV listener close did not complete within 2 seconds; continuing shutdown.");
        }

        try
        {
            if (!_acceptLoop.Wait(TimeSpan.FromSeconds(2)))
            {
                Console.Error.WriteLine("Warning: AF_HYPERV accept loop did not stop within 2 seconds; continuing shutdown.");
            }
        }
        catch
        {
            // Best-effort listener shutdown.
        }

        _stop.Dispose();
    }

    private static Task CloseSocketAndCleanupAsync(nint socket)
    {
        return Task.Run(() =>
        {
            _ = WindowsSockets.closesocket(socket);
            WindowsSockets.Cleanup();
        });
    }

    private void AcceptLoop(bool echo, CancellationToken cancellationToken)
    {
        while (!cancellationToken.IsCancellationRequested)
        {
            var client = WindowsSockets.accept(_listenSocket, nint.Zero, nint.Zero);
            if (client == InvalidSocket)
            {
                var error = WindowsSockets.LastError();
                if (error == WsaWouldBlock)
                {
                    cancellationToken.WaitHandle.WaitOne(AcceptPollInterval);
                    continue;
                }

                if (!cancellationToken.IsCancellationRequested)
                {
                    Console.Error.WriteLine($"Warning: AF_HYPERV accept failed: {WindowsSockets.FormatError(error)}");
                }

                break;
            }

            try
            {
                WindowsSockets.SetNonBlocking(client, enabled: false);
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"Warning: AF_HYPERV client setup failed: {ex.Message}");
                _ = WindowsSockets.closesocket(client);
                continue;
            }

            _ = Task.Run(() => HandleClient(client, echo, cancellationToken), cancellationToken);
        }
    }

    private static void HandleClient(nint client, bool echo, CancellationToken cancellationToken)
    {
        var buffer = ArrayPool<byte>.Shared.Rent(16 * 1024);
        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var received = WindowsSockets.recv(client, buffer, buffer.Length, 0);
                if (received == 0)
                {
                    break;
                }

                if (received == SocketError)
                {
                    Console.Error.WriteLine($"Warning: AF_HYPERV recv failed: {WindowsSockets.LastErrorMessage()}");
                    break;
                }

                var bytes = buffer.AsSpan(0, received).ToArray();
                Console.WriteLine($"HVSOCK received {received} byte(s): {Convert.ToHexString(bytes)}");

                if (echo)
                {
                    var sent = 0;
                    while (sent < received)
                    {
                        var rc = WindowsSockets.send(client, buffer.AsSpan(sent, received - sent).ToArray(), received - sent, 0);
                        if (rc == SocketError)
                        {
                            Console.Error.WriteLine($"Warning: AF_HYPERV send failed: {WindowsSockets.LastErrorMessage()}");
                            return;
                        }

                        sent += rc;
                    }
                }
            }
        }
        finally
        {
            ArrayPool<byte>.Shared.Return(buffer);
            _ = WindowsSockets.closesocket(client);
        }
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct SockAddrHv
    {
        public ushort Family;
        public ushort Reserved;
        public Guid VmId;
        public Guid ServiceId;

        public static SockAddrHv Create(Guid vmId, Guid serviceId) => new()
        {
            Family = AfHyperV,
            Reserved = 0,
            VmId = vmId,
            ServiceId = serviceId,
        };
    }

    private static class WindowsSockets
    {
        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int WSAStartup(ushort versionRequested, [Out] byte[] data);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int WSACleanup();

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int WSAGetLastError();

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern nint WSASocketW(int addressFamily, int socketType, int protocol, nint protocolInfo, uint group, uint flags);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int bind(nint socket, ref SockAddrHv name, int nameLength);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int listen(nint socket, int backlog);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int ioctlsocket(nint socket, int command, ref uint argp);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern nint accept(nint socket, nint address, nint addressLength);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int recv(nint socket, byte[] buffer, int length, int flags);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int send(nint socket, byte[] buffer, int length, int flags);

        [DllImport("Ws2_32.dll", ExactSpelling = true)]
        internal static extern int closesocket(nint socket);

        public static void Startup()
        {
            var data = new byte[512];
            var rc = WSAStartup(0x0202, data);
            if (rc != 0)
            {
                throw new Win32Exception(rc, "WSAStartup failed.");
            }
        }

        public static void Cleanup() => _ = WSACleanup();

        public static Exception Exception(string operation) => new Win32Exception(WSAGetLastError(), operation);

        public static int LastError() => WSAGetLastError();

        public static string LastErrorMessage() => FormatError(LastError());

        public static string FormatError(int error) => $"{error} ({new Win32Exception(error).Message})";

        public static void SetNonBlocking(nint socket, bool enabled)
        {
            const int fionbio = unchecked((int)0x8004667E);
            var mode = enabled ? 1u : 0u;
            if (ioctlsocket(socket, fionbio, ref mode) == SocketError)
            {
                throw Exception("ioctlsocket(FIONBIO)");
            }
        }
    }
}
