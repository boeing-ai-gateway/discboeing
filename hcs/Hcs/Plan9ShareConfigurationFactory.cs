using System.Text.Json;
using System.Text.Json.Serialization;

namespace HcsLinuxVmLauncher;

internal static class Plan9ShareConfigurationFactory
{
    private const int Plan9ShareFlagsReadOnly = 0x00000001;
    private const int Plan9ShareFlagsLinuxMetadata = 0x00000004;
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    public static string BuildAdd(Plan9ShareOption share) => Build("Add", ShareSettings(share, includePath: true));

    public static string BuildRemove(Plan9ShareOption share) => Build("Remove", ShareSettings(share, includePath: false));

    private static string Build(string requestType, object settings)
    {
        var document = new
        {
            ResourcePath = "VirtualMachine/Devices/Plan9/Shares",
            RequestType = requestType,
            Settings = settings,
        };

        return JsonSerializer.Serialize(document, JsonOptions);
    }

    private static object ShareSettings(Plan9ShareOption share, bool includePath) => new
    {
        Name = share.Name,
        AccessName = share.Name,
        Path = includePath ? share.HostPath : null,
        Port = Plan9ShareOption.DefaultPort,
        Flags = includePath ? Flags(share) : (int?)null,
    };

    private static int Flags(Plan9ShareOption share)
    {
        var flags = Plan9ShareFlagsLinuxMetadata;
        if (share.ReadOnly)
        {
            flags |= Plan9ShareFlagsReadOnly;
        }

        return flags;
    }
}
