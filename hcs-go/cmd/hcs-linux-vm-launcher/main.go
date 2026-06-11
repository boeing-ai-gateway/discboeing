package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/obot-platform/discobot/hcs-go/internal/cli"
	"github.com/obot-platform/discobot/hcs-go/internal/hcn"
	"github.com/obot-platform/discobot/hcs-go/internal/hcs"
	"github.com/obot-platform/discobot/hcs-go/internal/hvsocket"
	"github.com/obot-platform/discobot/hcs-go/internal/networking"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	options, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, cli.HelpText())
		return 2
	}
	if options.Help {
		fmt.Print(cli.HelpText())
		return 0
	}
	if err := options.Validate(!options.DryRun); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	hcsJSON, err := hcs.BuildConfiguration(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if options.DryRun {
		if err := printDryRun(options, hcsJSON); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	return launch(options, hcsJSON)
}

func printDryRun(options cli.Options, hcsJSON string) error {
	fmt.Println("# HCS compute system JSON")
	fmt.Println(hcsJSON)
	fmt.Println()
	fmt.Printf("# Hyper-V socket service GUID for VSOCK port %d\n", options.VsockPort)
	fmt.Println(options.VsockServiceID())

	switch options.NetworkMode {
	case cli.NetworkHCNNAT:
		networkJSON, err := hcn.BuildNetwork(options)
		if err != nil {
			return err
		}
		endpointJSON, err := hcn.BuildEndpoint(options)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("# HCN network JSON")
		fmt.Println(networkJSON)
		fmt.Println()
		fmt.Println("# HCN endpoint JSON")
		fmt.Println(endpointJSON)
	case cli.NetworkUserVsock:
		fmt.Println()
		fmt.Println("# gvproxy user-mode networking")
		printGvproxySummary(options)
	}

	for _, share := range options.Plan9Shares {
		shareJSON, err := hcs.BuildPlan9ShareAdd(share)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("# HCS Plan9 share add JSON for '%s'\n", share.Name)
		fmt.Println(shareJSON)
		fmt.Println()
		printPlan9ShareSummary(share)
	}
	return nil
}

func launch(options cli.Options, hcsJSON string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var accessGrant *hcs.VMAccessGrant
	var nat *hcn.NATConnection
	var computeSystem hcs.ComputeSystem
	var hvServer *hvsocket.Server
	var gvproxy *networking.GvproxyProcess
	var gvproxyBridge *hvsocket.TCPProxy

	defer func() {
		if hvServer != nil {
			_ = hvServer.Close()
		}
		if computeSystem != nil {
			fmt.Println("Terminating VM...")
			if err := computeSystem.Terminate(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: VM termination failed: %v\n", err)
			}
			_ = computeSystem.Close()
		}
		if nat != nil {
			_ = nat.Close()
		}
		if gvproxyBridge != nil {
			_ = gvproxyBridge.Close()
		}
		if gvproxy != nil {
			_ = gvproxy.Close()
		}
		if accessGrant != nil {
			_ = accessGrant.Close()
		}
	}()

	if !options.SkipGrantVMAccess {
		fmt.Println("Granting the VM access to kernel, initrd, and VHDX files...")
		var err error
		accessGrant, err = hcs.GrantVMAccess(options)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	gvproxyTCPPort := 0
	if options.NetworkMode == cli.NetworkUserVsock {
		port, err := allocateLoopbackTCPPort()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		gvproxyTCPPort = port
		fmt.Printf("Starting gvproxy user-mode networking on %s...\n", networking.TCPListenURI(gvproxyTCPPort))
		gvproxy, err = networking.StartGvproxy(options.GvproxyPath, gvproxyTCPPort)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	fmt.Printf("Creating HCS VM %s...\n", options.VMID)
	system, err := hcs.CreateComputeSystem(options.VMID, hcsJSON)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	computeSystem = system

	if options.NetworkMode == cli.NetworkUserVsock {
		gvproxyBridge, err = hvsocket.StartTCPProxy(options.VMID, options.GvproxyServiceID(), "127.0.0.1", gvproxyTCPPort, ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Bridging Hyper-V socket service %s (VSOCK port %d) to gvproxy.\n", options.GvproxyServiceID(), options.GvproxyVsockPort)
	}

	fmt.Println("Starting VM...")
	if err := computeSystem.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	for _, share := range options.Plan9Shares {
		fmt.Printf("Adding Plan9 share '%s' for %s (%s)...\n", share.Name, share.HostPath, readOnlyText(share.ReadOnly))
		shareJSON, err := hcs.BuildPlan9ShareAdd(share)
		if err != nil {
			fmt.Fprintf(os.Stderr, "building Plan9 share JSON failed: %v\n", err)
			return 1
		}
		if err := computeSystem.Modify(shareJSON); err != nil {
			fmt.Fprintf(os.Stderr, "adding Plan9 share failed: %v\n", err)
			return 1
		}
		printPlan9ShareSummary(share)
	}

	if options.NetworkMode == cli.NetworkHCNNAT {
		fmt.Printf("Creating HCN NAT network '%s' and endpoint...\n", options.EffectiveNATName())
		var err error
		nat, err = hcn.CreateNAT(options)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Attaching HCN endpoint %s to the VM...\n", options.NATEndpointID)
		if err := nat.Attach(computeSystem); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		printEndpointSummary(nat.Properties)
	}

	if options.ListenVsock {
		var err error
		hvServer, err = hvsocket.StartServer(options.VMID, options.VsockPort, options.EchoVsock, ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Listening on Hyper-V socket service %s (VSOCK port %d).\n", options.VsockServiceID(), options.VsockPort)
	}

	fmt.Println()
	fmt.Println("VM is running. Press Ctrl+C or send process termination to stop it.")
	<-ctx.Done()
	return 0
}

func printPlan9ShareSummary(share cli.Plan9Share) {
	guestPath := "/mnt/" + share.Name
	fmt.Printf("Plan9 share '%s' (%s) host path: %s\n", share.Name, readOnlyText(share.ReadOnly), share.HostPath)
	fmt.Println("Guest mount hint:")
	fmt.Printf("  mkdir -p %s\n", guestPath)
	fmt.Printf("  %s\n", share.MountCommand(guestPath))
}

func printGvproxySummary(options cli.Options) {
	fmt.Println("No HCN network or VM NIC will be attached.")
	fmt.Printf("gvproxy command: %s\n", networking.CommandLine(options.GvproxyPath))
	fmt.Printf("Launcher bridge: %s -> tcp://127.0.0.1:<auto>\n", networking.HVSocketListenURI(options.VMID, options.GvproxyServiceID()))
	fmt.Printf("Hyper-V socket service GUID: %s\n", options.GvproxyServiceID())
	fmt.Printf("Guest static config: discobot=ip=%s,netmask=%s,gateway=%s,dns=%s\n", options.UsernetIP, options.UsernetNetmask, options.UsernetGateway, options.UsernetDNS)
	fmt.Println("Guest prerequisite: run gvforwarder in the VM with hv_sock and tun support available.")
}

func printEndpointSummary(properties hcn.EndpointProperties) {
	fmt.Printf("Endpoint MAC address: %s\n", properties.MacAddress)
	if properties.IPAddress != "" {
		fmt.Printf("Endpoint IP address: %s\n", properties.IPAddress)
	}
	if properties.GatewayAddress != "" {
		fmt.Printf("Endpoint gateway: %s\n", properties.GatewayAddress)
	}
}

func allocateLoopbackTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func readOnlyText(readOnly bool) string {
	if readOnly {
		return "read-only"
	}
	return "read/write"
}
