using Microsoft.Win32.SafeHandles;

namespace HcsLinuxVmLauncher;

internal sealed class SafeHcsOperationHandle : SafeHandleZeroOrMinusOneIsInvalid
{
    private SafeHcsOperationHandle()
        : base(ownsHandle: true)
    {
    }

    protected override bool ReleaseHandle()
    {
        NativeMethods.HcsCloseOperation(handle);
        return true;
    }
}

internal sealed class SafeHcsSystemHandle : SafeHandleZeroOrMinusOneIsInvalid
{
    private SafeHcsSystemHandle()
        : base(ownsHandle: true)
    {
    }

    protected override bool ReleaseHandle()
    {
        NativeMethods.HcsCloseComputeSystem(handle);
        return true;
    }
}

internal sealed class SafeHcnNetworkHandle : SafeHandleZeroOrMinusOneIsInvalid
{
    private SafeHcnNetworkHandle()
        : base(ownsHandle: true)
    {
    }

    protected override bool ReleaseHandle()
    {
        NativeMethods.HcnCloseNetwork(handle);
        return true;
    }
}

internal sealed class SafeHcnEndpointHandle : SafeHandleZeroOrMinusOneIsInvalid
{
    private SafeHcnEndpointHandle()
        : base(ownsHandle: true)
    {
    }

    protected override bool ReleaseHandle()
    {
        NativeMethods.HcnCloseEndpoint(handle);
        return true;
    }
}
