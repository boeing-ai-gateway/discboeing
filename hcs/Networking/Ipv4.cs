namespace HcsLinuxVmLauncher;

internal readonly record struct Ipv4Address(uint Value)
{
    public static Ipv4Address Parse(string text)
    {
        var parts = text.Split('.', StringSplitOptions.TrimEntries);
        if (parts.Length != 4)
        {
            throw new ArgumentException($"Invalid IPv4 address: {text}");
        }

        uint value = 0;
        foreach (var part in parts)
        {
            if (!byte.TryParse(part, out var octet))
            {
                throw new ArgumentException($"Invalid IPv4 address: {text}");
            }

            value = (value << 8) | octet;
        }

        return new Ipv4Address(value);
    }

    public override string ToString() => string.Create(15, Value, static (span, value) =>
    {
        var text = $"{(value >> 24) & 0xff}.{(value >> 16) & 0xff}.{(value >> 8) & 0xff}.{value & 0xff}";
        text.AsSpan().CopyTo(span);
    }).TrimEnd('\0');
}

internal readonly record struct Ipv4Cidr(Ipv4Address Network, int PrefixLength)
{
    public static Ipv4Cidr Parse(string text)
    {
        var slash = text.IndexOf('/');
        if (slash <= 0 || slash == text.Length - 1)
        {
            throw new ArgumentException($"Invalid IPv4 CIDR: {text}");
        }

        var address = Ipv4Address.Parse(text[..slash]);
        if (!int.TryParse(text[(slash + 1)..], out var prefix) || prefix is < 1 or > 30)
        {
            throw new ArgumentException($"Invalid IPv4 CIDR prefix length: {text}");
        }

        return new Ipv4Cidr(address, prefix);
    }
}
