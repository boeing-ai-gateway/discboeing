namespace HcsLinuxVmLauncher;

internal static class HvSocketPorts
{
    private static readonly byte[] VsockTemplateTail = { 0xbd, 0x58, 0x64, 0x00, 0x6a, 0x79, 0x86, 0xd3 };

    public static Guid PortToServiceId(int port)
    {
        if (port is < 1 or > 0x7fffffff)
        {
            throw new ArgumentOutOfRangeException(nameof(port), "VSOCK ports must fit in the first DWORD of the Hyper-V socket service GUID.");
        }

        return new Guid(port, unchecked((short)0xfacb), 0x11e6, VsockTemplateTail);
    }
}
