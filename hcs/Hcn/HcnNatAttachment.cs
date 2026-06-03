namespace HcsLinuxVmLauncher;

internal sealed class HcnNatAttachment : IDisposable
{
    private readonly Guid _networkId;
    private readonly Guid _endpointId;
    private readonly bool _deleteNetwork;
    private readonly SafeHcnNetworkHandle _network;
    private readonly SafeHcnEndpointHandle _endpoint;
    private bool _disposed;

    private HcnNatAttachment(
        Guid networkId,
        Guid endpointId,
        bool deleteNetwork,
        SafeHcnNetworkHandle network,
        SafeHcnEndpointHandle endpoint,
        HcnEndpointProperties properties,
        string networkJson,
        string endpointJson,
        string adapterJson)
    {
        _networkId = networkId;
        _endpointId = endpointId;
        _deleteNetwork = deleteNetwork;
        _network = network;
        _endpoint = endpoint;
        Properties = properties;
        NetworkJson = networkJson;
        EndpointJson = endpointJson;
        AdapterJson = adapterJson;
    }

    public HcnEndpointProperties Properties { get; }
    public string NetworkJson { get; }
    public string EndpointJson { get; }
    public string AdapterJson { get; }

    public static HcnNatAttachment Create(CliOptions options)
    {
        var networkJson = HcnConfigurationFactory.BuildNetwork(options);
        var network = CreateOrOpenNetwork(options.NatNetworkId, networkJson, out var createdNetwork);

        try
        {
            var endpointJson = HcnConfigurationFactory.BuildEndpoint(options);
            var endpoint = CreateEndpoint(network, options.NatEndpointId, endpointJson);

            try
            {
                var propertiesJson = QueryEndpointProperties(endpoint);
                var properties = HcnEndpointProperties.Parse(options.NatEndpointId, propertiesJson);
                var adapterJson = HcnConfigurationFactory.BuildNetworkAdapterModify(properties);

                return new HcnNatAttachment(
                    options.NatNetworkId,
                    options.NatEndpointId,
                    createdNetwork,
                    network,
                    endpoint,
                    properties,
                    networkJson,
                    endpointJson,
                    adapterJson);
            }
            catch
            {
                endpoint.Dispose();
                TryDeleteEndpoint(options.NatEndpointId);
                throw;
            }
        }
        catch
        {
            network.Dispose();
            if (createdNetwork)
            {
                TryDeleteNetwork(options.NatNetworkId);
            }

            throw;
        }
    }

    public void Attach(HcsComputeSystem computeSystem) => computeSystem.Modify(AdapterJson);

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;
        _endpoint.Dispose();
        TryDeleteEndpoint(_endpointId);
        _network.Dispose();

        if (_deleteNetwork)
        {
            TryDeleteNetwork(_networkId);
        }
    }

    private static SafeHcnNetworkHandle CreateOrOpenNetwork(Guid id, string settings, out bool created)
    {
        var hr = NativeMethods.HcnCreateNetwork(id, settings, out var network, out var errorRecordPointer);
        var errorRecord = NativeMethods.ConsumeNativeString(errorRecordPointer);
        if (!HResults.Failed(hr))
        {
            created = true;
            return network;
        }

        network.Dispose();

        // HNS returns a specific HCN_E_NETWORK_ALREADY_EXISTS code, but the value has changed across
        // SDK surfaces. Try opening the requested network before surfacing the original create error.
        var openHr = NativeMethods.HcnOpenNetwork(id, out var openedNetwork, out var openErrorPointer);
        var openError = NativeMethods.ConsumeNativeString(openErrorPointer);
        if (!HResults.Failed(openHr))
        {
            created = false;
            return openedNetwork;
        }

        openedNetwork.Dispose();
        throw new HcnException("HcnCreateNetwork", hr, errorRecord ?? openError);
    }

    private static SafeHcnEndpointHandle CreateEndpoint(SafeHcnNetworkHandle network, Guid id, string settings)
    {
        var hr = NativeMethods.HcnCreateEndpoint(network, id, settings, out var endpoint, out var errorRecordPointer);
        var errorRecord = NativeMethods.ConsumeNativeString(errorRecordPointer);
        if (HResults.Failed(hr))
        {
            endpoint.Dispose();
            throw new HcnException("HcnCreateEndpoint", hr, errorRecord);
        }

        return endpoint;
    }

    private static string QueryEndpointProperties(SafeHcnEndpointHandle endpoint)
    {
        var hr = NativeMethods.HcnQueryEndpointProperties(endpoint, null, out var propertiesPointer, out var errorRecordPointer);
        var properties = NativeMethods.ConsumeNativeString(propertiesPointer);
        var errorRecord = NativeMethods.ConsumeNativeString(errorRecordPointer);
        if (HResults.Failed(hr))
        {
            throw new HcnException("HcnQueryEndpointProperties", hr, errorRecord);
        }

        return properties ?? throw new HcnException("HcnQueryEndpointProperties", unchecked((int)0x80004005), "No endpoint properties were returned.");
    }

    private static void TryDeleteEndpoint(Guid id)
    {
        try
        {
            var hr = NativeMethods.HcnDeleteEndpoint(id, out var errorRecordPointer);
            var errorRecord = NativeMethods.ConsumeNativeString(errorRecordPointer);
            if (HResults.Failed(hr))
            {
                Console.Error.WriteLine($"Warning: HcnDeleteEndpoint({id:D}) failed with HRESULT {HResults.Format(hr)}: {errorRecord}");
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"Warning: HcnDeleteEndpoint({id:D}) failed: {ex.Message}");
        }
    }

    private static void TryDeleteNetwork(Guid id)
    {
        try
        {
            var hr = NativeMethods.HcnDeleteNetwork(id, out var errorRecordPointer);
            var errorRecord = NativeMethods.ConsumeNativeString(errorRecordPointer);
            if (HResults.Failed(hr))
            {
                Console.Error.WriteLine($"Warning: HcnDeleteNetwork({id:D}) failed with HRESULT {HResults.Format(hr)}: {errorRecord}");
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"Warning: HcnDeleteNetwork({id:D}) failed: {ex.Message}");
        }
    }
}
