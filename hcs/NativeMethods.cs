using System.ComponentModel;
using System.Runtime.InteropServices;

namespace HcsLinuxVmLauncher;

internal static class NativeMethods
{
    internal const uint Infinite = 0xffffffff;

    [DllImport("kernel32.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern nint LocalFree(nint hMem);

    [DllImport("ComputeCore.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern SafeHcsOperationHandle HcsCreateOperation(nint context, nint callback);

    [DllImport("ComputeCore.dll", ExactSpelling = true)]
    internal static extern void HcsCloseOperation(nint operation);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsCreateComputeSystem(
        string id,
        string configuration,
        SafeHcsOperationHandle operation,
        nint securityDescriptor,
        out SafeHcsSystemHandle computeSystem);

    [DllImport("ComputeCore.dll", ExactSpelling = true)]
    internal static extern void HcsCloseComputeSystem(nint computeSystem);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsStartComputeSystem(
        SafeHcsSystemHandle computeSystem,
        SafeHcsOperationHandle operation,
        string? options);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsTerminateComputeSystem(
        SafeHcsSystemHandle computeSystem,
        SafeHcsOperationHandle operation,
        string? options);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsModifyComputeSystem(
        SafeHcsSystemHandle computeSystem,
        SafeHcsOperationHandle operation,
        string configuration,
        nint identity);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsGetComputeSystemProperties(
        SafeHcsSystemHandle computeSystem,
        SafeHcsOperationHandle operation,
        string? propertyQuery);

    [DllImport("ComputeCore.dll", ExactSpelling = true)]
    internal static extern int HcsWaitForOperationResult(
        SafeHcsOperationHandle operation,
        uint timeoutMs,
        out nint resultDocument);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsGrantVmAccess(string vmId, string filePath);

    [DllImport("ComputeCore.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcsRevokeVmAccess(string vmId, string filePath);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcnCreateNetwork(
        in Guid id,
        string settings,
        out SafeHcnNetworkHandle network,
        out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern int HcnOpenNetwork(
        in Guid id,
        out SafeHcnNetworkHandle network,
        out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern void HcnCloseNetwork(nint network);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern int HcnDeleteNetwork(in Guid id, out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcnQueryNetworkProperties(
        SafeHcnNetworkHandle network,
        string? query,
        out nint properties,
        out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcnCreateEndpoint(
        SafeHcnNetworkHandle network,
        in Guid id,
        string settings,
        out SafeHcnEndpointHandle endpoint,
        out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern int HcnOpenEndpoint(
        in Guid id,
        out SafeHcnEndpointHandle endpoint,
        out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern void HcnCloseEndpoint(nint endpoint);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true)]
    internal static extern int HcnDeleteEndpoint(in Guid id, out nint errorRecord);

    [DllImport("ComputeNetwork.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    internal static extern int HcnQueryEndpointProperties(
        SafeHcnEndpointHandle endpoint,
        string? query,
        out nint properties,
        out nint errorRecord);

    internal static string? ConsumeNativeString(nint value)
    {
        if (value == nint.Zero)
        {
            return null;
        }

        try
        {
            return Marshal.PtrToStringUni(value);
        }
        finally
        {
            _ = LocalFree(value);
        }
    }

    internal static Win32Exception LastWin32Exception(string operation) =>
        new(Marshal.GetLastWin32Error(), operation);
}
