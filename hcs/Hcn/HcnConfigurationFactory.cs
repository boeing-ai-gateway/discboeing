using System.Text.Json;
using System.Text.Json.Serialization;

namespace HcsLinuxVmLauncher;

internal static class HcnConfigurationFactory
{
    private const int NetworkFlagEnableDns = 1;
    private const int NetworkFlagEnableDhcp = 2;
    private const int NetworkFlagEnableNonPersistent = 8;
    private const int NetworkFlagDisableHostPort = 1024;

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    public static string BuildNetwork(CliOptions options)
    {
        var flags = NetworkFlagEnableDns | NetworkFlagEnableNonPersistent;
        if (options.NatEnableDhcp)
        {
            flags |= NetworkFlagEnableDhcp;
        }

        if (options.NatDisableHostPort)
        {
            flags |= NetworkFlagDisableHostPort;
        }

        var subnets = new[]
        {
            new
            {
                GatewayAddress = options.NatGateway,
                AddressPrefix = options.NatSubnet,
                IpSubnets = new[]
                {
                    new { IpAddressPrefix = options.NatSubnet },
                },
            },
        };

        var document = new
        {
            Name = options.EffectiveNatName,
            Type = "ICS",
            IsolateSwitch = true,
            Flags = flags,
            Subnets = subnets,
        };

        return JsonSerializer.Serialize(document, JsonOptions);
    }

    public static string BuildEndpoint(CliOptions options)
    {
        var ipConfigurations = string.IsNullOrWhiteSpace(options.NatVmIp)
            ? Array.Empty<object>()
            : new object[] { new { IpAddress = options.NatVmIp } };

        var document = new
        {
            SchemaVersion = new { Major = 2, Minor = 16 },
            HostComputeNetwork = options.NatNetworkId,
            Policies = new object[]
            {
                new
                {
                    Type = "PortName",
                    Settings = new { Name = string.Empty },
                },
            },
            IpConfigurations = ipConfigurations,
        };

        return JsonSerializer.Serialize(document, JsonOptions);
    }

    public static string BuildNetworkAdapterModify(HcnEndpointProperties endpoint)
    {
        var document = new
        {
            ResourcePath = $"VirtualMachine/Devices/NetworkAdapters/{endpoint.Id:D}",
            RequestType = "Add",
            Settings = new
            {
                EndpointId = endpoint.Id,
                InstanceId = endpoint.Id,
                MacAddress = endpoint.MacAddress,
            },
        };

        return JsonSerializer.Serialize(document, JsonOptions);
    }
}
