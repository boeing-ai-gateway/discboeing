namespace HcsLinuxVmLauncher;

internal sealed class HcsComputeSystem : IDisposable
{
    private bool _terminated;

    private HcsComputeSystem(Guid id, SafeHcsSystemHandle handle)
    {
        Id = id;
        Handle = handle;
    }

    public Guid Id { get; }
    public SafeHcsSystemHandle Handle { get; }

    public static HcsComputeSystem Create(Guid id, string configuration)
    {
        using var operation = HcsOperation.Create();
        var hr = NativeMethods.HcsCreateComputeSystem(id.ToString("D"), configuration, operation.Handle, nint.Zero, out var handle);
        if (HResults.Failed(hr))
        {
            handle.Dispose();
            throw new HcsException("HcsCreateComputeSystem", hr);
        }

        try
        {
            _ = operation.Wait("HcsCreateComputeSystem");
            return new HcsComputeSystem(id, handle);
        }
        catch
        {
            handle.Dispose();
            throw;
        }
    }

    public void Start()
    {
        using var operation = HcsOperation.Create();
        var hr = NativeMethods.HcsStartComputeSystem(Handle, operation.Handle, null);
        if (HResults.Failed(hr))
        {
            throw new HcsException("HcsStartComputeSystem", hr);
        }

        _ = operation.Wait("HcsStartComputeSystem");
    }

    public void Modify(string configuration)
    {
        using var operation = HcsOperation.Create();
        var hr = NativeMethods.HcsModifyComputeSystem(Handle, operation.Handle, configuration, nint.Zero);
        if (HResults.Failed(hr))
        {
            throw new HcsException("HcsModifyComputeSystem", hr);
        }

        _ = operation.Wait("HcsModifyComputeSystem");
    }

    public string? GetProperties(string? query = null)
    {
        using var operation = HcsOperation.Create();
        var hr = NativeMethods.HcsGetComputeSystemProperties(Handle, operation.Handle, query);
        if (HResults.Failed(hr))
        {
            throw new HcsException("HcsGetComputeSystemProperties", hr);
        }

        return operation.Wait("HcsGetComputeSystemProperties");
    }

    public void Terminate()
    {
        if (_terminated || Handle.IsInvalid || Handle.IsClosed)
        {
            return;
        }

        using var operation = HcsOperation.Create();
        var hr = NativeMethods.HcsTerminateComputeSystem(Handle, operation.Handle, null);
        if (HResults.Failed(hr))
        {
            throw new HcsException("HcsTerminateComputeSystem", hr);
        }

        _ = operation.Wait("HcsTerminateComputeSystem");
        _terminated = true;
    }

    public void Dispose() => Handle.Dispose();
}
