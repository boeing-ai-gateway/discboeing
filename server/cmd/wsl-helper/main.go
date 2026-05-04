package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/server/internal/windows/virtualdisk"
)

var createDynamicVHD = virtualdisk.CreateDynamicVHDX

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	resultFile, commandArgs, err := parseGlobalArgs(args)
	if err != nil {
		return finish(resultFile, err)
	}
	if len(commandArgs) == 0 {
		return finish(resultFile, errors.New(usage()))
	}

	ctx := context.Background()
	switch commandArgs[0] {
	case "create-vhd":
		err = runCreateVHD(ctx, commandArgs[1:])
	case "mount-vhd":
		err = runMountVHD(ctx, commandArgs[1:])
	case "mount-vhd-bare":
		err = runMountVHDBare(ctx, commandArgs[1:])
	case "unmount-vhd":
		err = runUnmountVHD(ctx, commandArgs[1:])
	case "help", "--help", "-h":
		fmt.Fprintln(os.Stdout, usage())
		if err := writeResultFile(resultFile, ""); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return 0
	default:
		err = fmt.Errorf("unknown command %q\n%s", commandArgs[0], usage())
	}

	return finish(resultFile, err)
}

func parseGlobalArgs(args []string) (string, []string, error) {
	resultFile := ""
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--result-file":
			i++
			if i >= len(args) {
				return "", nil, fmt.Errorf("--result-file requires <path>")
			}
			resultFile = strings.TrimSpace(args[i])
			if resultFile == "" {
				return "", nil, fmt.Errorf("--result-file requires <path>")
			}
		default:
			remaining = append(remaining, args[i:]...)
			return resultFile, remaining, nil
		}
	}
	return resultFile, remaining, nil
}

func finish(resultFile string, err error) int {
	if err == nil {
		if resultErr := writeResultFile(resultFile, ""); resultErr != nil {
			fmt.Fprintln(os.Stderr, resultErr.Error())
			return 1
		}
		return 0
	}
	message := err.Error()
	fmt.Fprintln(os.Stderr, message)
	if resultErr := writeResultFile(resultFile, message); resultErr != nil {
		fmt.Fprintln(os.Stderr, resultErr.Error())
	}
	return 1
}

func writeResultFile(path string, contents string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		return fmt.Errorf("write helper result file %q: %w", path, err)
	}
	return nil
}

func runCreateVHD(_ context.Context, args []string) error {
	path, sizeGB, err := parseCreateVHDArgs(args)
	if err != nil {
		return err
	}
	err = createDynamicVHD(path, uint64(sizeGB)*1024*1024*1024)
	if err != nil {
		return fmt.Errorf("create VHD %q: %w", path, err)
	}
	return nil
}

func runMountVHD(ctx context.Context, args []string) error {
	path, name, fsType, err := parseMountVHDArgs(args)
	if err != nil {
		return err
	}
	_, err = runCommand(ctx, "wsl.exe", "--mount", "--vhd", path, "--name", name, "--type", fsType)
	if err != nil {
		return fmt.Errorf("mount VHD %q: %w", path, err)
	}
	return nil
}

func runMountVHDBare(ctx context.Context, args []string) error {
	path, err := parsePathOnlyArgs("mount-vhd-bare", args)
	if err != nil {
		return err
	}
	_, err = runCommand(ctx, "wsl.exe", "--mount", "--vhd", path, "--bare")
	if err != nil {
		return fmt.Errorf("mount VHD %q in bare mode: %w", path, err)
	}
	return nil
}

func runUnmountVHD(ctx context.Context, args []string) error {
	path, err := parsePathOnlyArgs("unmount-vhd", args)
	if err != nil {
		return err
	}
	_, err = runCommand(ctx, "wsl.exe", "--unmount", path)
	if err != nil {
		return fmt.Errorf("unmount VHD %q: %w", path, err)
	}
	return nil
}

func parseCreateVHDArgs(args []string) (string, int, error) {
	path := ""
	sizeGB := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("create-vhd requires --path <path>")
			}
			path = strings.TrimSpace(args[i])
		case "--size-gb":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("create-vhd requires --size-gb <value>")
			}
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return "", 0, fmt.Errorf("create-vhd requires a positive integer for --size-gb")
			}
			sizeGB = parsed
		default:
			return "", 0, fmt.Errorf("unexpected create-vhd argument %q", args[i])
		}
	}
	if path == "" {
		return "", 0, fmt.Errorf("create-vhd requires --path <path>")
	}
	if sizeGB <= 0 {
		return "", 0, fmt.Errorf("create-vhd requires --size-gb <positive integer>")
	}
	return path, sizeGB, nil
}

func parseMountVHDArgs(args []string) (string, string, string, error) {
	path := ""
	name := ""
	fsType := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("mount-vhd requires --path <path>")
			}
			path = strings.TrimSpace(args[i])
		case "--name":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("mount-vhd requires --name <name>")
			}
			name = strings.TrimSpace(args[i])
		case "--type":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("mount-vhd requires --type <filesystem>")
			}
			fsType = strings.TrimSpace(args[i])
		default:
			return "", "", "", fmt.Errorf("unexpected mount-vhd argument %q", args[i])
		}
	}
	if path == "" {
		return "", "", "", fmt.Errorf("mount-vhd requires --path <path>")
	}
	if name == "" {
		return "", "", "", fmt.Errorf("mount-vhd requires --name <name>")
	}
	if fsType == "" {
		return "", "", "", fmt.Errorf("mount-vhd requires --type <filesystem>")
	}
	return path, name, fsType, nil
}

func parsePathOnlyArgs(command string, args []string) (string, error) {
	if len(args) != 2 || args[0] != "--path" || strings.TrimSpace(args[1]) == "" {
		return "", fmt.Errorf("%s requires --path <path>", command)
	}
	return strings.TrimSpace(args[1]), nil
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

func usage() string {
	return strings.TrimSpace(`usage:
  discobot-wsl-helper create-vhd --path <path> --size-gb <size>
  discobot-wsl-helper mount-vhd --path <path> --name <name> --type <filesystem>
  discobot-wsl-helper mount-vhd-bare --path <path>
  discobot-wsl-helper unmount-vhd --path <path>`)
}
