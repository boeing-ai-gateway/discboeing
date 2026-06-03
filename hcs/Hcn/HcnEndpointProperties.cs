using System.Text.Json;

namespace HcsLinuxVmLauncher;

internal sealed record HcnEndpointProperties(
    Guid Id,
    string MacAddress,
    string? IpAddress,
    string? GatewayAddress,
    byte? PrefixLength)
{
    public static HcnEndpointProperties Parse(Guid fallbackId, string json)
    {
        using var document = JsonDocument.Parse(json);
        var root = document.RootElement;

        var id = fallbackId;
        if (TryGetProperty(root, "ID", out var idElement) || TryGetProperty(root, "Id", out idElement))
        {
            var idText = idElement.GetString();
            if (!string.IsNullOrWhiteSpace(idText) && Guid.TryParse(idText, out var parsed))
            {
                id = parsed;
            }
        }

        var macAddress = GetString(root, "MacAddress") ?? GetString(root, "MACAddress");
        if (string.IsNullOrWhiteSpace(macAddress))
        {
            throw new InvalidOperationException($"HCN endpoint properties did not contain a MacAddress: {json}");
        }

        var ipAddress = GetString(root, "IPAddress") ?? GetString(root, "IpAddress");
        var gatewayAddress = GetString(root, "GatewayAddress");
        byte? prefixLength = null;
        if (TryGetProperty(root, "PrefixLength", out var prefixElement) && prefixElement.TryGetByte(out var prefix))
        {
            prefixLength = prefix;
        }

        return new HcnEndpointProperties(id, macAddress, ipAddress, gatewayAddress, prefixLength);
    }

    private static string? GetString(JsonElement root, string propertyName)
    {
        return TryGetProperty(root, propertyName, out var value) && value.ValueKind == JsonValueKind.String
            ? value.GetString()
            : null;
    }

    private static bool TryGetProperty(JsonElement root, string propertyName, out JsonElement value)
    {
        foreach (var property in root.EnumerateObject())
        {
            if (string.Equals(property.Name, propertyName, StringComparison.OrdinalIgnoreCase))
            {
                value = property.Value;
                return true;
            }
        }

        value = default;
        return false;
    }
}
