namespace HcsLinuxVmLauncher;

internal sealed class HcsOperation : IDisposable
{
    private readonly SafeHcsOperationHandle _handle;

    private HcsOperation(SafeHcsOperationHandle handle)
    {
        _handle = handle;
    }

    public SafeHcsOperationHandle Handle => _handle;

    public static HcsOperation Create()
    {
        var handle = NativeMethods.HcsCreateOperation(nint.Zero, nint.Zero);
        if (handle is null || handle.IsInvalid)
        {
            throw NativeMethods.LastWin32Exception("HcsCreateOperation");
        }

        return new HcsOperation(handle);
    }

    public string? Wait(string operation)
    {
        var hr = NativeMethods.HcsWaitForOperationResult(_handle, NativeMethods.Infinite, out var resultDocument);
        var result = NativeMethods.ConsumeNativeString(resultDocument);
        if (HResults.Failed(hr))
        {
            throw new HcsException(operation, hr, result);
        }

        return result;
    }

    public void Dispose() => _handle.Dispose();
}
