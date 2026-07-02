package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/hvsocket"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/resources"
)

type NetworkMode string

const (
	NetworkNone      NetworkMode = "none"
	NetworkHCNNAT    NetworkMode = "hcn-nat"
	NetworkUserVsock NetworkMode = "user-vsock"
)

type Plan9Share struct {
	Name     string
	HostPath string
	ReadOnly bool
}

const Plan9DefaultPort = 564

func (s Plan9Share) MountCommand(guestPath string) string {
	readonly := ""
	if s.ReadOnly {
		readonly = ",ro"
	}
	return fmt.Sprintf("mount -t 9p -o trans=virtio,version=9p2000.L,aname=%s%s %s %s", s.Name, readonly, s.Name, guestPath)
}

type Options struct {
	VMID                       uuid.UUID
	Owner                      string
	KernelPath                 string
	InitrdPath                 *string
	RootDiskPath               string
	DataDiskPath               string
	RootDevice                 string
	RootFileSystem             *string
	KernelCommandLineOverride  string
	AppendKernelCommandLine    string
	MemoryMB                   int
	ProcessorCount             int
	VsockPort                  int
	GvproxyVsockPort           int
	GvproxyPath                string
	UsernetIP                  string
	UsernetNetmask             string
	UsernetGateway             string
	UsernetDNS                 string
	ListenVsock                bool
	EchoVsock                  bool
	HvSocketSecurityDescriptor string
	ConsolePipeName            string
	Plan9Shares                []Plan9Share
	NetworkMode                NetworkMode
	NATNetworkID               uuid.UUID
	NATEndpointID              uuid.UUID
	NATName                    string
	NATSubnet                  string
	NATGateway                 string
	NATVMIP                    string
	NATEnableDHCP              bool
	NATDisableHostPort         bool
	SkipGrantVMAccess          bool
	DryRun                     bool
	Help                       bool
}

func DefaultOptions() Options {
	vmID := uuid.New()
	initrd := defaultWSLToolPath("initrd.img")
	rootFSType := "ext4"
	return Options{
		VMID:                       vmID,
		Owner:                      "HcsLinuxVmLauncher",
		KernelPath:                 defaultWSLToolPath("kernel"),
		InitrdPath:                 &initrd,
		RootDevice:                 "/dev/sda1",
		RootFileSystem:             &rootFSType,
		MemoryMB:                   resources.DefaultMemoryMB(),
		ProcessorCount:             resources.DefaultProcessorCount(),
		VsockPort:                  5000,
		GvproxyVsockPort:           1024,
		GvproxyPath:                "gvproxy.exe",
		UsernetIP:                  "192.168.127.2",
		UsernetNetmask:             "255.255.255.0",
		UsernetGateway:             "192.168.127.1",
		UsernetDNS:                 "192.168.127.1",
		HvSocketSecurityDescriptor: "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;IU)",
		NetworkMode:                NetworkHCNNAT,
		NATNetworkID:               uuid.New(),
		NATEndpointID:              uuid.New(),
		NATSubnet:                  "172.31.240.0/20",
		NATGateway:                 "172.31.240.1",
		NATEnableDHCP:              true,
	}
}

func Parse(args []string) (options Options, err error) {
	options = DefaultOptions()
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("%v", value)
		}
	}()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			options.Help = true
		case "--dry-run":
			options.DryRun = true
		case "--id":
			id, err := uuid.Parse(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.VMID = id
		case "--owner":
			options.Owner = requireValue(args, &i, arg)
		case "--kernel":
			options.KernelPath = requireValue(args, &i, arg)
		case "--initrd":
			value := requireValue(args, &i, arg)
			options.InitrdPath = &value
		case "--no-initrd":
			options.InitrdPath = nil
		case "--root":
			options.RootDiskPath = requireValue(args, &i, arg)
		case "--data":
			options.DataDiskPath = requireValue(args, &i, arg)
		case "--root-device":
			options.RootDevice = requireValue(args, &i, arg)
		case "--root-fstype":
			value := requireValue(args, &i, arg)
			options.RootFileSystem = &value
		case "--no-root-fstype":
			options.RootFileSystem = nil
		case "--kernel-cmdline":
			options.KernelCommandLineOverride = requireValue(args, &i, arg)
		case "--append-kernel-cmdline":
			options.AppendKernelCommandLine = appendText(options.AppendKernelCommandLine, requireValue(args, &i, arg))
		case "--memory-mb":
			value, err := strconv.Atoi(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.MemoryMB = value
		case "--ram-gb", "--memory-gb":
			gb, err := strconv.ParseFloat(requireValue(args, &i, arg), 64)
			if err != nil {
				return options, err
			}
			mb, err := resources.MemoryGBToMB(gb)
			if err != nil {
				return options, err
			}
			options.MemoryMB = mb
		case "--processors":
			value, err := strconv.Atoi(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.ProcessorCount = value
		case "--vsock-port":
			value, err := strconv.Atoi(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.VsockPort = value
		case "--gvproxy":
			options.GvproxyPath = requireValue(args, &i, arg)
		case "--gvproxy-vsock-port":
			value, err := strconv.Atoi(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.GvproxyVsockPort = value
		case "--usernet-ip":
			options.UsernetIP = requireValue(args, &i, arg)
		case "--usernet-netmask":
			options.UsernetNetmask = requireValue(args, &i, arg)
		case "--usernet-gateway":
			options.UsernetGateway = requireValue(args, &i, arg)
		case "--usernet-dns":
			options.UsernetDNS = requireValue(args, &i, arg)
		case "--listen-vsock":
			options.ListenVsock = true
		case "--echo-vsock":
			options.EchoVsock = true
		case "--hvsocket-sddl":
			options.HvSocketSecurityDescriptor = requireValue(args, &i, arg)
		case "--console-pipe":
			options.ConsolePipeName = requireValue(args, &i, arg)
		case "--share":
			share, err := parsePlan9Share(requireValue(args, &i, arg), true)
			if err != nil {
				return options, err
			}
			options.Plan9Shares = append(options.Plan9Shares, share)
		case "--share-rw":
			share, err := parsePlan9Share(requireValue(args, &i, arg), false)
			if err != nil {
				return options, err
			}
			options.Plan9Shares = append(options.Plan9Shares, share)
		case "--network":
			mode, err := parseNetworkMode(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.NetworkMode = mode
		case "--nat-network-id":
			id, err := uuid.Parse(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.NATNetworkID = id
		case "--nat-endpoint-id":
			id, err := uuid.Parse(requireValue(args, &i, arg))
			if err != nil {
				return options, err
			}
			options.NATEndpointID = id
		case "--nat-name":
			options.NATName = requireValue(args, &i, arg)
		case "--nat-subnet":
			options.NATSubnet = requireValue(args, &i, arg)
		case "--nat-gateway":
			options.NATGateway = requireValue(args, &i, arg)
		case "--nat-vm-ip":
			options.NATVMIP = requireValue(args, &i, arg)
		case "--nat-disable-dhcp":
			options.NATEnableDHCP = false
		case "--nat-disable-host-port":
			options.NATDisableHostPort = true
		case "--skip-grant-vm-access":
			options.SkipGrantVMAccess = true
		default:
			return options, fmt.Errorf("unknown argument: %s", arg)
		}
	}
	if strings.TrimSpace(options.ConsolePipeName) == "" {
		options.ConsolePipeName = fmt.Sprintf(`\\.\pipe\hcs-linux-vm-%s-hvc0`, strings.ReplaceAll(options.VMID.String(), "-", ""))
	}
	return options, nil
}

func (o Options) Validate(validateFiles bool) error {
	if o.RootDiskPath == "" {
		return errors.New("missing required --root <root.vhdx> argument")
	}
	if o.DataDiskPath == "" {
		return errors.New("missing required --data <data.vhdx> argument")
	}
	if o.MemoryMB < 128 {
		return errors.New("--memory-mb must be at least 128")
	}
	if o.ProcessorCount < 1 {
		return errors.New("--processors must be at least 1")
	}
	if _, err := hvsocket.PortToServiceID(o.VsockPort); err != nil {
		return fmt.Errorf("--vsock-port must be between 1 and 2147483647")
	}
	if _, err := hvsocket.PortToServiceID(o.GvproxyVsockPort); err != nil {
		return fmt.Errorf("--gvproxy-vsock-port must be between 1 and 2147483647")
	}
	if o.NetworkMode == NetworkUserVsock && o.ListenVsock && o.VsockPort == o.GvproxyVsockPort {
		return errors.New("--listen-vsock cannot use the same port as --gvproxy-vsock-port")
	}
	if o.NetworkMode == NetworkUserVsock {
		for _, ip := range []string{o.UsernetIP, o.UsernetNetmask, o.UsernetGateway, o.UsernetDNS} {
			if net.ParseIP(ip) == nil || strings.Contains(ip, ":") {
				return fmt.Errorf("invalid IPv4 address: %s", ip)
			}
		}
	}
	if o.NetworkMode == NetworkHCNNAT {
		if _, _, err := net.ParseCIDR(o.NATSubnet); err != nil {
			return fmt.Errorf("invalid IPv4 CIDR: %s", o.NATSubnet)
		}
		for _, ip := range []string{o.NATGateway, o.NATVMIP} {
			if ip != "" && (net.ParseIP(ip) == nil || strings.Contains(ip, ":")) {
				return fmt.Errorf("invalid IPv4 address: %s", ip)
			}
		}
	}
	if !validateFiles {
		return nil
	}
	for _, file := range o.FilesNeedingVMAccess() {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("required VM asset was not found: %s", file)
		}
	}
	for _, share := range o.Plan9Shares {
		info, err := os.Stat(share.HostPath)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("Plan9 share directory was not found: %s", share.HostPath)
		}
	}
	if o.NetworkMode == NetworkUserVsock {
		path, err := resolveExecutablePath(o.GvproxyPath)
		if err != nil {
			return err
		}
		o.GvproxyPath = path
	}
	return nil
}

func (o Options) EffectiveNATName() string {
	if o.NATName != "" {
		return o.NATName
	}
	name := "hcs-linux-vm-" + strings.ReplaceAll(o.VMID.String(), "-", "")
	if len(name) > 29 {
		return name[:29]
	}
	return name
}

func (o Options) VsockServiceID() uuid.UUID {
	return hvsocket.MustPortToServiceID(o.VsockPort)
}

func (o Options) GvproxyServiceID() uuid.UUID {
	return hvsocket.MustPortToServiceID(o.GvproxyVsockPort)
}

func (o Options) FilesNeedingVMAccess() []string {
	files := []string{o.KernelPath}
	if o.InitrdPath != nil && strings.TrimSpace(*o.InitrdPath) != "" {
		files = append(files, *o.InitrdPath)
	}
	if strings.TrimSpace(o.RootDiskPath) != "" {
		files = append(files, o.RootDiskPath)
	}
	if strings.TrimSpace(o.DataDiskPath) != "" {
		files = append(files, o.DataDiskPath)
	}
	return files
}

func HelpText() string {
	return strings.Join([]string{
		"Launch a Linux utility VM through the Windows Host Compute System API.",
		"",
		"Required:",
		"  hcs-linux-vm-launcher --root C:\\vm\\rootfs.vhdx --data C:\\vm\\data.vhdx [options]",
		"",
		"Important options:",
		"  --kernel <path>                 WSL2 kernel path. Default: %ProgramFiles%\\WSL\\tools\\kernel",
		"  --initrd <path>                 WSL2 initrd path. Default: %ProgramFiles%\\WSL\\tools\\initrd.img",
		"  --no-initrd                     Do not pass an initrd to LinuxKernelDirect.",
		"  --root-device <dev>             Kernel root= value. Default: /dev/sda1",
		"  --root-fstype <type>            Kernel rootfstype= value. Default: ext4",
		"  --kernel-cmdline <text>         Replace the generated kernel command line.",
		"  --append-kernel-cmdline <text>  Append extra kernel command-line options.",
		"  --memory-mb <n>                 VM memory in MiB. Default: 50% of host physical memory",
		"  --ram-gb <n>                    VM memory in GiB. Alias: --memory-gb",
		"  --processors <n>                vCPU count. Default: host logical processor count",
		"  --vsock-port <n>                Linux AF_VSOCK port exposed via Hyper-V socket. Default: 5000",
		"  --gvproxy <path>                gvproxy executable for --network user-vsock. Default: gvproxy.exe",
		"  --gvproxy-vsock-port <n>        VSOCK port for gvproxy user networking. Default: 1024",
		"  --listen-vsock                  Open a host HVSOCK listener and print received bytes.",
		"  --echo-vsock                    Echo bytes received by --listen-vsock.",
		"  --share <name=host-dir>         Add a read-only Plan9/9P host directory share.",
		"  --share-rw <name=host-dir>      Add a read/write Plan9/9P host directory share.",
		"  --network hcn-nat|none|user-vsock",
		"  --dry-run                       Print generated HCS JSON and exit.",
		"",
	}, "\n")
}

func requireValue(args []string, index *int, option string) string {
	if *index+1 >= len(args) {
		panic(fmt.Sprintf("%s requires a value", option))
	}
	*index++
	return args[*index]
}

func parsePlan9Share(value string, readOnly bool) (Plan9Share, error) {
	sep := strings.IndexByte(value, '=')
	if sep <= 0 || sep == len(value)-1 {
		return Plan9Share{}, errors.New("Plan9 share must use <name=host-dir> syntax")
	}
	name := value[:sep]
	hostPath := value[sep+1:]
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "/\\: \t\r\n") {
		return Plan9Share{}, fmt.Errorf("invalid Plan9 share name '%s'. Use a simple mount tag without whitespace, slashes, or colons", name)
	}
	if strings.TrimSpace(hostPath) == "" || !isFullyQualifiedPath(hostPath) {
		return Plan9Share{}, fmt.Errorf("Plan9 share path must be fully qualified: %s", hostPath)
	}
	return Plan9Share{Name: name, HostPath: hostPath, ReadOnly: readOnly}, nil
}

func parseNetworkMode(value string) (NetworkMode, error) {
	switch strings.ToLower(value) {
	case "none":
		return NetworkNone, nil
	case "hcn-nat", "nat":
		return NetworkHCNNAT, nil
	case "user-vsock", "gvproxy", "slirp":
		return NetworkUserVsock, nil
	default:
		return "", fmt.Errorf("unsupported --network value '%s'. Use 'hcn-nat', 'none', or 'user-vsock'", value)
	}
}

func appendText(existing, value string) string {
	if strings.TrimSpace(existing) == "" {
		return value
	}
	return existing + " " + value
}

func defaultWSLToolPath(fileName string) string {
	programFiles := os.Getenv("ProgramFiles")
	if strings.TrimSpace(programFiles) == "" {
		programFiles = `C:\Program Files`
	}
	return programFiles + `\WSL\tools\` + fileName
}

func isFullyQualifiedPath(path string) bool {
	if runtime.GOOS == "windows" {
		return filepath.IsAbs(path)
	}
	if filepath.IsAbs(path) {
		return true
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	if strings.HasPrefix(path, `\\`) {
		return true
	}
	return false
}

func resolveExecutablePath(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", errors.New("--gvproxy requires a non-empty executable path")
	}
	if filepath.IsAbs(executable) || strings.ContainsAny(executable, `/\`) {
		if _, err := os.Stat(executable); err == nil {
			return executable, nil
		}
		return "", fmt.Errorf("gvproxy executable was not found: %s", executable)
	}
	pathEnv := os.Getenv("PATH")
	exts := []string{""}
	if filepath.Ext(executable) == "" {
		exts = strings.Split(os.Getenv("PATHEXT"), ";")
		if len(exts) == 0 || exts[0] == "" {
			exts = []string{".COM", ".EXE", ".BAT", ".CMD"}
		}
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		for _, ext := range exts {
			candidate := filepath.Join(dir, executable+ext)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("gvproxy executable was not found on PATH: %s", executable)
}
