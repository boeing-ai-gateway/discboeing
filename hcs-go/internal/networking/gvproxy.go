package networking

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type GvproxyProcess struct {
	cmd *exec.Cmd
}

type GvproxyOptions interface {
	GvproxyServiceID() fmt.Stringer
}

func StartGvproxy(path string, tcpPort int) (*GvproxyProcess, error) {
	cmd := exec.Command(path, "-debug", "-listen", TCPListenURI(tcpPort))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go copyPrefixed(stdout)
	go copyPrefixed(stderr)
	return &GvproxyProcess{cmd: cmd}, nil
}

func (p *GvproxyProcess) Close() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
	return nil
}

func CommandLine(path string) string {
	return fmt.Sprintf("%s -debug -listen tcp://127.0.0.1:<auto>", quote(path))
}

func CommandLineWithPort(path string, tcpPort int) string {
	return fmt.Sprintf("%s -debug -listen %s", quote(path), TCPListenURI(tcpPort))
}

func HVSocketListenURI(vmID, serviceID fmt.Stringer) string {
	return fmt.Sprintf("hvsock://%s/%s", vmID.String(), serviceID.String())
}

func TCPListenURI(tcpPort int) string {
	return fmt.Sprintf("tcp://127.0.0.1:%d", tcpPort)
}

func copyPrefixed(reader io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			fmt.Printf("gvproxy: %s", string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func quote(value string) string {
	if strings.Contains(value, " ") {
		return `"` + value + `"`
	}
	return value
}
