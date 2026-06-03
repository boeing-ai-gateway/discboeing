namespace HcsLinuxVmLauncher;

internal sealed class VmAccessGrant : IDisposable
{
    private readonly List<string> _grantedFiles = new();
    private readonly string _vmId;
    private bool _disposed;

    private VmAccessGrant(string vmId)
    {
        _vmId = vmId;
    }

    public static VmAccessGrant Grant(CliOptions options)
    {
        var grant = new VmAccessGrant(options.VmIdString);
        foreach (var file in options.FilesNeedingVmAccess().Distinct(StringComparer.OrdinalIgnoreCase))
        {
            var hr = NativeMethods.HcsGrantVmAccess(options.VmIdString, file);
            if (HResults.Failed(hr))
            {
                grant.Dispose();
                throw new HcsException("HcsGrantVmAccess", hr, file);
            }

            grant._grantedFiles.Add(file);
        }

        return grant;
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;
        foreach (var file in _grantedFiles.AsEnumerable().Reverse())
        {
            try
            {
                var hr = NativeMethods.HcsRevokeVmAccess(_vmId, file);
                if (HResults.Failed(hr))
                {
                    Console.Error.WriteLine($"Warning: HcsRevokeVmAccess failed for '{file}' with HRESULT {HResults.Format(hr)}.");
                }
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"Warning: HcsRevokeVmAccess failed for '{file}': {ex.Message}");
            }
        }
    }
}
