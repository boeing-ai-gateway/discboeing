namespace HcsLinuxVmLauncher;

internal sealed record Plan9ShareOption(string Name, string HostPath, bool ReadOnly)
{
    public const int DefaultPort = 564;

    public string MountCommand(string guestPath) =>
        $"mount -t 9p -o trans=virtio,version=9p2000.L,aname={Name}{(ReadOnly ? ",ro" : string.Empty)} {Name} {guestPath}";
}
