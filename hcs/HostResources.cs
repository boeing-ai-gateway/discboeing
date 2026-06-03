using System.Runtime.InteropServices;

namespace HcsLinuxVmLauncher;

internal static class HostResources
{
    private const long Mib = 1024L * 1024L;
    private const int MinimumMemoryMb = 128;

    public static int DefaultProcessorCount() => Math.Max(1, Environment.ProcessorCount);

    public static int DefaultMemoryMb()
    {
        var halfHostMemoryBytes = TotalPhysicalMemoryBytes() / 2;
        if (halfHostMemoryBytes <= 0)
        {
            return 2048;
        }

        var memoryMb = (int)Math.Min(int.MaxValue, halfHostMemoryBytes / Mib);
        return AlignMemoryMb(Math.Max(MinimumMemoryMb, memoryMb));
    }

    public static int MemoryGbToMb(decimal memoryGb)
    {
        if (memoryGb <= 0)
        {
            throw new ArgumentException("Memory size must be greater than zero.");
        }

        var memoryMb = memoryGb * 1024m;
        if (memoryMb > int.MaxValue)
        {
            throw new ArgumentOutOfRangeException(nameof(memoryGb), "Memory size is too large.");
        }

        return AlignMemoryMb((int)Math.Ceiling(memoryMb));
    }

    public static int AlignMemoryMb(int memoryMb) => memoryMb & ~1;

    private static long TotalPhysicalMemoryBytes()
    {
        if (OperatingSystem.IsWindows())
        {
            var status = new MemoryStatusEx
            {
                Length = (uint)Marshal.SizeOf<MemoryStatusEx>(),
            };

            if (GlobalMemoryStatusEx(ref status))
            {
                return status.TotalPhys > long.MaxValue ? long.MaxValue : (long)status.TotalPhys;
            }
        }

        return TryReadProcMeminfo() ?? GC.GetGCMemoryInfo().TotalAvailableMemoryBytes;
    }

    private static long? TryReadProcMeminfo()
    {
        const string memTotalPrefix = "MemTotal:";
        try
        {
            if (!File.Exists("/proc/meminfo"))
            {
                return null;
            }

            foreach (var line in File.ReadLines("/proc/meminfo"))
            {
                if (!line.StartsWith(memTotalPrefix, StringComparison.Ordinal))
                {
                    continue;
                }

                var parts = line[memTotalPrefix.Length..].Split(' ', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
                if (parts.Length > 0 && long.TryParse(parts[0], out var kib))
                {
                    return kib * 1024L;
                }
            }
        }
        catch
        {
        }

        return null;
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    private static extern bool GlobalMemoryStatusEx(ref MemoryStatusEx buffer);

    [StructLayout(LayoutKind.Sequential)]
    private struct MemoryStatusEx
    {
        public uint Length;
        public uint MemoryLoad;
        public ulong TotalPhys;
        public ulong AvailPhys;
        public ulong TotalPageFile;
        public ulong AvailPageFile;
        public ulong TotalVirtual;
        public ulong AvailVirtual;
        public ulong AvailExtendedVirtual;
    }
}
