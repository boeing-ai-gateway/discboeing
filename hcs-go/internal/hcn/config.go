package hcn

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/obot-platform/discobot/hcs-go/internal/cli"
)

const (
	networkFlagEnableDNS           = 1
	networkFlagEnableDHCP          = 2
	networkFlagEnableNonPersistent = 8
	networkFlagDisableHostPort     = 1024
)

type EndpointProperties struct {
	ID             uuid.UUID
	MacAddress     string
	IPAddress      string
	GatewayAddress string
	PrefixLength   *byte
}

func BuildNetwork(options cli.Options) (string, error) {
	flags := networkFlagEnableDNS | networkFlagEnableNonPersistent
	if options.NATEnableDHCP {
		flags |= networkFlagEnableDHCP
	}
	if options.NATDisableHostPort {
		flags |= networkFlagDisableHostPort
	}
	document := map[string]any{
		"Name":          options.EffectiveNATName(),
		"Type":          "ICS",
		"IsolateSwitch": true,
		"Flags":         flags,
		"Subnets": []any{
			map[string]any{
				"GatewayAddress": options.NATGateway,
				"AddressPrefix":  options.NATSubnet,
				"IpSubnets": []any{
					map[string]any{"IpAddressPrefix": options.NATSubnet},
				},
			},
		},
	}
	return marshalIndented(document)
}

func BuildEndpoint(options cli.Options) (string, error) {
	ipConfigurations := []any{}
	if options.NATVMIP != "" {
		ipConfigurations = append(ipConfigurations, map[string]any{"IpAddress": options.NATVMIP})
	}
	document := map[string]any{
		"SchemaVersion":      map[string]any{"Major": 2, "Minor": 16},
		"HostComputeNetwork": options.NATNetworkID,
		"Policies": []any{
			map[string]any{
				"Type":     "PortName",
				"Settings": map[string]any{"Name": ""},
			},
		},
		"IpConfigurations": ipConfigurations,
	}
	return marshalIndented(document)
}

func BuildNetworkAdapterModify(endpoint EndpointProperties) (string, error) {
	document := map[string]any{
		"ResourcePath": fmt.Sprintf("VirtualMachine/Devices/NetworkAdapters/%s", endpoint.ID.String()),
		"RequestType":  "Add",
		"Settings": map[string]any{
			"EndpointId": endpoint.ID,
			"InstanceId": endpoint.ID,
			"MacAddress": endpoint.MacAddress,
		},
	}
	return marshalIndented(document)
}

func ParseEndpointProperties(fallbackID uuid.UUID, text string) (EndpointProperties, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return EndpointProperties{}, err
	}
	props := EndpointProperties{ID: fallbackID}
	if idText, ok := getString(raw, "ID", "Id"); ok {
		if id, err := uuid.Parse(idText); err == nil {
			props.ID = id
		}
	}
	if mac, ok := getString(raw, "MacAddress", "MACAddress"); ok {
		props.MacAddress = mac
	}
	if props.MacAddress == "" {
		return EndpointProperties{}, fmt.Errorf("HCN endpoint properties did not contain a MacAddress: %s", text)
	}
	if ip, ok := getString(raw, "IPAddress", "IpAddress"); ok {
		props.IPAddress = ip
	}
	if gateway, ok := getString(raw, "GatewayAddress"); ok {
		props.GatewayAddress = gateway
	}
	if prefix, ok := raw["PrefixLength"]; ok {
		if f, ok := prefix.(float64); ok {
			b := byte(f)
			props.PrefixLength = &b
		}
	}
	return props, nil
}

func getString(raw map[string]any, names ...string) (string, bool) {
	for key, value := range raw {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				text, ok := value.(string)
				return text, ok
			}
		}
	}
	return "", false
}

func marshalIndented(value any) (string, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
