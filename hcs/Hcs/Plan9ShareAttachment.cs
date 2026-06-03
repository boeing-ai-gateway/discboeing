namespace HcsLinuxVmLauncher;

internal sealed class Plan9ShareAttachment : IDisposable
{
    private readonly HcsComputeSystem _computeSystem;
    private readonly Plan9ShareOption _share;
    private bool _disposed;

    private Plan9ShareAttachment(HcsComputeSystem computeSystem, Plan9ShareOption share)
    {
        _computeSystem = computeSystem;
        _share = share;
    }

    public static Plan9ShareAttachment Add(HcsComputeSystem computeSystem, Plan9ShareOption share)
    {
        computeSystem.Modify(Plan9ShareConfigurationFactory.BuildAdd(share));
        return new Plan9ShareAttachment(computeSystem, share);
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;
        try
        {
            _computeSystem.Modify(Plan9ShareConfigurationFactory.BuildRemove(_share));
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"Warning: removing Plan9 share '{_share.Name}' failed: {ex.Message}");
        }
    }
}
