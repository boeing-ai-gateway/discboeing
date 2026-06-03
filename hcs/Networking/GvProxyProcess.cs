using System.Diagnostics;

namespace HcsLinuxVmLauncher;

internal sealed class GvProxyProcess : IDisposable
{
    private readonly Process _process;
    private bool _disposed;

    private GvProxyProcess(Process process)
    {
        _process = process;
    }

    public static GvProxyProcess Start(CliOptions options, int tcpPort)
    {
        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = options.GvproxyPath,
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
            },
            EnableRaisingEvents = true,
        };

        process.StartInfo.ArgumentList.Add("-debug");
        process.StartInfo.ArgumentList.Add("-listen");
        process.StartInfo.ArgumentList.Add(TcpListenUri(tcpPort));

        process.OutputDataReceived += (_, eventArgs) => PrintLine(eventArgs.Data);
        process.ErrorDataReceived += (_, eventArgs) => PrintLine(eventArgs.Data);

        try
        {
            if (!process.Start())
            {
                process.Dispose();
                throw new InvalidOperationException("gvproxy did not start.");
            }

            process.BeginOutputReadLine();
            process.BeginErrorReadLine();

            if (process.WaitForExit(500))
            {
                var exitCode = process.ExitCode;
                process.Dispose();
                throw new InvalidOperationException($"gvproxy exited immediately with code {exitCode}.");
            }

            return new GvProxyProcess(process);
        }
        catch
        {
            process.Dispose();
            throw;
        }
    }

    public static string CommandLine(CliOptions options) => $"{Quote(options.GvproxyPath)} -debug -listen {TcpListenUriText}";

    public static string CommandLine(CliOptions options, int tcpPort) => $"{Quote(options.GvproxyPath)} -debug -listen {TcpListenUri(tcpPort)}";

    public static string HvSocketListenUri(CliOptions options) => $"hvsock://{options.VmId:D}/{options.GvproxyServiceId:D}";

    public static string TcpListenUri(int tcpPort) => $"tcp://127.0.0.1:{tcpPort}";

    private const string TcpListenUriText = "tcp://127.0.0.1:<auto>";

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;
        try
        {
            if (!_process.HasExited)
            {
                _process.Kill(entireProcessTree: true);
                _process.WaitForExit(5000);
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"Warning: stopping gvproxy failed: {ex.Message}");
        }
        finally
        {
            _process.Dispose();
        }
    }

    private static void PrintLine(string? line)
    {
        if (!string.IsNullOrWhiteSpace(line))
        {
            Console.Error.WriteLine($"gvproxy: {line}");
        }
    }

    private static string Quote(string value) => value.Contains(' ') ? $"\"{value}\"" : value;
}
