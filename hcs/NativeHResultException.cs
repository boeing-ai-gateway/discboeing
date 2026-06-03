namespace HcsLinuxVmLauncher;

internal static class HResults
{
    public static bool Failed(int hr) => hr < 0;

    public static string Format(int hr) => $"0x{unchecked((uint)hr):X8}";
}

internal class NativeHResultException : Exception
{
    public NativeHResultException(string operation, int hresult, string? details = null)
        : base(BuildMessage(operation, hresult, details))
    {
        Operation = operation;
        HResultCode = hresult;
        Details = details;
        HResult = hresult;
    }

    public string Operation { get; }
    public int HResultCode { get; }
    public string? Details { get; }

    private static string BuildMessage(string operation, int hresult, string? details)
    {
        var message = $"{operation} failed with HRESULT {HResults.Format(hresult)}";
        if (!string.IsNullOrWhiteSpace(details))
        {
            message += $": {details}";
        }

        return message;
    }
}

internal sealed class HcsException : NativeHResultException
{
    public HcsException(string operation, int hresult, string? resultDocument = null)
        : base(operation, hresult, resultDocument)
    {
    }
}

internal sealed class HcnException : NativeHResultException
{
    public HcnException(string operation, int hresult, string? errorRecord = null)
        : base(operation, hresult, errorRecord)
    {
    }
}
