package hcs

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/cli"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/resources"
)

func BuildConfiguration(options cli.Options) (string, error) {
	kernelCommandLine := BuildKernelCommandLine(options)
	hvSocketConfig := map[string]any{
		"DefaultBindSecurityDescriptor":    options.HvSocketSecurityDescriptor,
		"DefaultConnectSecurityDescriptor": options.HvSocketSecurityDescriptor,
	}

	serviceTable := buildHvSocketServiceTable(options)
	if len(serviceTable) > 0 {
		hvSocketConfig["ServiceTable"] = serviceTable
	}

	attachments := map[string]any{
		"0": attachment(options.RootDiskPath, true),
		"1": attachment(options.DataDiskPath, false),
	}

	devices := map[string]any{
		"ComPorts": map[string]any{},
		"Scsi": map[string]any{
			"0": map[string]any{"Attachments": attachments},
		},
		"HvSocket": map[string]any{"HvSocketConfig": hvSocketConfig},
		"Plan9":    map[string]any{},
		"Battery":  map[string]any{},
	}

	if strings.TrimSpace(options.ConsolePipeName) != "" {
		devices["VirtioSerial"] = map[string]any{
			"Ports": map[string]any{
				"0": map[string]any{
					"NamedPipe":      options.ConsolePipeName,
					"Name":           "hvc0",
					"ConsoleSupport": true,
				},
			},
		}
	}

	linuxKernelDirect := map[string]any{
		"KernelFilePath": options.KernelPath,
		"KernelCmdLine":  kernelCommandLine,
	}
	if options.InitrdPath != nil {
		linuxKernelDirect["InitRdPath"] = *options.InitrdPath
	}

	document := map[string]any{
		"Owner":                             options.Owner,
		"SchemaVersion":                     map[string]any{"Major": 2, "Minor": 3},
		"ShouldTerminateOnLastHandleClosed": true,
		"VirtualMachine": map[string]any{
			"StopOnReset": true,
			"Chipset": map[string]any{
				"UseUtc":            true,
				"LinuxKernelDirect": linuxKernelDirect,
			},
			"ComputeTopology": map[string]any{
				"Memory": map[string]any{
					"SizeInMB":                 resources.AlignMemoryMB(options.MemoryMB),
					"AllowOvercommit":          true,
					"EnableDeferredCommit":     true,
					"EnableColdDiscardHint":    true,
					"HighMmioBaseInMB":         49152,
					"HighMmioGapInMB":          16384,
					"HostingProcessNameSuffix": "HcsLinuxVmLauncher",
				},
				"Processor": map[string]any{"Count": options.ProcessorCount},
			},
			"Devices":      devices,
			"DebugOptions": map[string]any{},
		},
	}
	return marshalIndented(document)
}

func BuildKernelCommandLine(options cli.Options) string {
	if strings.TrimSpace(options.KernelCommandLineOverride) != "" {
		commandLine := options.KernelCommandLineOverride
		if options.NetworkMode == cli.NetworkUserVsock {
			commandLine = appendPart(commandLine, buildDiscboeingKernelOption(options))
		}
		return appendPart(commandLine, options.AppendKernelCommandLine)
	}

	parts := []string{}
	if options.InitrdPath != nil && strings.TrimSpace(*options.InitrdPath) != "" {
		parts = append(parts, `initrd=\initrd.img`)
	}
	parts = append(parts, "root="+options.RootDevice)
	if options.RootFileSystem != nil && strings.TrimSpace(*options.RootFileSystem) != "" {
		parts = append(parts, "rootfstype="+*options.RootFileSystem)
	}
	parts = append(parts,
		"ro",
		"rootwait",
		"panic=1",
		"nr_cpus="+strconvItoa(options.ProcessorCount),
		"hv_utils.timesync_implicit=1",
		"console=hvc0",
		"earlyprintk=serial",
		"pty.legacy_count=0",
	)
	if options.NetworkMode == cli.NetworkUserVsock {
		parts = append(parts, buildDiscboeingKernelOption(options))
	}
	return appendPart(strings.Join(parts, " "), options.AppendKernelCommandLine)
}

func buildDiscboeingKernelOption(options cli.Options) string {
	return strings.Join([]string{
		"discboeing=ip=" + options.UsernetIP,
		"netmask=" + options.UsernetNetmask,
		"gateway=" + options.UsernetGateway,
		"dns=" + options.UsernetDNS,
	}, ",")
}

func buildHvSocketServiceTable(options cli.Options) map[string]any {
	services := map[string]any{}
	if options.NetworkMode == cli.NetworkUserVsock {
		addHvSocketService(services, options.GvproxyServiceID().String(), options.HvSocketSecurityDescriptor)
	}
	if options.ListenVsock {
		addHvSocketService(services, options.VsockServiceID().String(), options.HvSocketSecurityDescriptor)
	}
	return services
}

func addHvSocketService(services map[string]any, serviceID, securityDescriptor string) {
	services[serviceID] = map[string]any{
		"BindSecurityDescriptor":    securityDescriptor,
		"ConnectSecurityDescriptor": securityDescriptor,
	}
}

func attachment(path string, readOnly bool) map[string]any {
	return map[string]any{
		"Type":                     "VirtualDisk",
		"Path":                     path,
		"ReadOnly":                 readOnly,
		"SupportCompressedVolumes": true,
		"AlwaysAllowSparseFiles":   true,
		"SupportEncryptedFiles":    true,
	}
}

func appendPart(commandLine, append string) string {
	if strings.TrimSpace(append) == "" {
		return commandLine
	}
	return commandLine + " " + append
}

func marshalIndented(value any) (string, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func strconvItoa(value int) string {
	return strconv.FormatInt(int64(value), 10)
}
