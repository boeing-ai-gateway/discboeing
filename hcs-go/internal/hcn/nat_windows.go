//go:build windows

package hcn

import (
	"fmt"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/cli"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/hcs"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/winapi"
)

type NATConnection struct {
	networkID     uuid.UUID
	endpointID    uuid.UUID
	deleteNetwork bool
	network       windows.Handle
	endpoint      windows.Handle
	Properties    EndpointProperties
	NetworkJSON   string
	EndpointJSON  string
	AdapterJSON   string
}

func CreateNAT(options cli.Options) (*NATConnection, error) {
	networkJSON, err := BuildNetwork(options)
	if err != nil {
		return nil, err
	}
	network, created, err := createOrOpenNetwork(options.NATNetworkID, networkJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			winapi.ProcHcnCloseNetwork.Call(uintptr(network))
			if created {
				tryDeleteNetwork(options.NATNetworkID)
			}
		}
	}()

	endpointJSON, err := BuildEndpoint(options)
	if err != nil {
		return nil, err
	}
	endpoint, err := createEndpoint(network, options.NATEndpointID, endpointJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			winapi.ProcHcnCloseEndpoint.Call(uintptr(endpoint))
			tryDeleteEndpoint(options.NATEndpointID)
		}
	}()

	propertiesJSON, err := queryEndpointProperties(endpoint)
	if err != nil {
		return nil, err
	}
	properties, err := ParseEndpointProperties(options.NATEndpointID, propertiesJSON)
	if err != nil {
		return nil, err
	}
	adapterJSON, err := BuildNetworkAdapterModify(properties)
	if err != nil {
		return nil, err
	}
	return &NATConnection{
		networkID:     options.NATNetworkID,
		endpointID:    options.NATEndpointID,
		deleteNetwork: created,
		network:       network,
		endpoint:      endpoint,
		Properties:    properties,
		NetworkJSON:   networkJSON,
		EndpointJSON:  endpointJSON,
		AdapterJSON:   adapterJSON,
	}, nil
}

func (n *NATConnection) Attach(system hcs.ComputeSystem) error {
	return system.Modify(n.AdapterJSON)
}

func (n *NATConnection) Close() error {
	if n.endpoint != 0 {
		winapi.ProcHcnCloseEndpoint.Call(uintptr(n.endpoint))
		n.endpoint = 0
	}
	tryDeleteEndpoint(n.endpointID)
	if n.network != 0 {
		winapi.ProcHcnCloseNetwork.Call(uintptr(n.network))
		n.network = 0
	}
	if n.deleteNetwork {
		tryDeleteNetwork(n.networkID)
	}
	return nil
}

func createOrOpenNetwork(id uuid.UUID, settings string) (windows.Handle, bool, error) {
	settingsPtr, err := winapi.UTF16Ptr(settings)
	if err != nil {
		return 0, false, err
	}
	winID := winapi.GUIDFromUUID(id)
	var network windows.Handle
	var errorRecord uintptr
	hr, _, _ := winapi.ProcHcnCreateNetwork.Call(uintptr(unsafe.Pointer(&winID)), uintptr(unsafe.Pointer(settingsPtr)), uintptr(unsafe.Pointer(&network)), uintptr(unsafe.Pointer(&errorRecord)))
	errorText := winapi.ConsumeNativeString(errorRecord)
	if !winapi.Failed(hr) {
		return network, true, nil
	}
	if network != 0 {
		winapi.ProcHcnCloseNetwork.Call(uintptr(network))
	}
	var opened windows.Handle
	var openErrorRecord uintptr
	openHR, _, _ := winapi.ProcHcnOpenNetwork.Call(uintptr(unsafe.Pointer(&winID)), uintptr(unsafe.Pointer(&opened)), uintptr(unsafe.Pointer(&openErrorRecord)))
	openError := winapi.ConsumeNativeString(openErrorRecord)
	if !winapi.Failed(openHR) {
		return opened, false, nil
	}
	if opened != 0 {
		winapi.ProcHcnCloseNetwork.Call(uintptr(opened))
	}
	if errorText == "" {
		errorText = openError
	}
	return 0, false, winapi.HRESULTError("HcnCreateNetwork", hr, errorText)
}

func createEndpoint(network windows.Handle, id uuid.UUID, settings string) (windows.Handle, error) {
	settingsPtr, err := winapi.UTF16Ptr(settings)
	if err != nil {
		return 0, err
	}
	winID := winapi.GUIDFromUUID(id)
	var endpoint windows.Handle
	var errorRecord uintptr
	hr, _, _ := winapi.ProcHcnCreateEndpoint.Call(uintptr(network), uintptr(unsafe.Pointer(&winID)), uintptr(unsafe.Pointer(settingsPtr)), uintptr(unsafe.Pointer(&endpoint)), uintptr(unsafe.Pointer(&errorRecord)))
	errorText := winapi.ConsumeNativeString(errorRecord)
	if winapi.Failed(hr) {
		if endpoint != 0 {
			winapi.ProcHcnCloseEndpoint.Call(uintptr(endpoint))
		}
		return 0, winapi.HRESULTError("HcnCreateEndpoint", hr, errorText)
	}
	return endpoint, nil
}

func queryEndpointProperties(endpoint windows.Handle) (string, error) {
	var properties uintptr
	var errorRecord uintptr
	hr, _, _ := winapi.ProcHcnQueryEndpointProperties.Call(uintptr(endpoint), 0, uintptr(unsafe.Pointer(&properties)), uintptr(unsafe.Pointer(&errorRecord)))
	text := winapi.ConsumeNativeString(properties)
	errorText := winapi.ConsumeNativeString(errorRecord)
	if winapi.Failed(hr) {
		return "", winapi.HRESULTError("HcnQueryEndpointProperties", hr, errorText)
	}
	if text == "" {
		return "", winapi.HRESULTError("HcnQueryEndpointProperties", 0x80004005, "No endpoint properties were returned")
	}
	return text, nil
}

func tryDeleteEndpoint(id uuid.UUID) {
	winID := winapi.GUIDFromUUID(id)
	var errorRecord uintptr
	hr, _, _ := winapi.ProcHcnDeleteEndpoint.Call(uintptr(unsafe.Pointer(&winID)), uintptr(unsafe.Pointer(&errorRecord)))
	errorText := winapi.ConsumeNativeString(errorRecord)
	if winapi.Failed(hr) {
		fmt.Printf("Warning: HcnDeleteEndpoint(%s) failed with HRESULT 0x%08x: %s\n", id, uint32(hr), errorText)
	}
}

func tryDeleteNetwork(id uuid.UUID) {
	winID := winapi.GUIDFromUUID(id)
	var errorRecord uintptr
	hr, _, _ := winapi.ProcHcnDeleteNetwork.Call(uintptr(unsafe.Pointer(&winID)), uintptr(unsafe.Pointer(&errorRecord)))
	errorText := winapi.ConsumeNativeString(errorRecord)
	if winapi.Failed(hr) {
		fmt.Printf("Warning: HcnDeleteNetwork(%s) failed with HRESULT 0x%08x: %s\n", id, uint32(hr), errorText)
	}
}
