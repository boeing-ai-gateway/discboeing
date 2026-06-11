//go:build windows

package hcs

import (
	"runtime"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"

	"github.com/obot-platform/discobot/hcs-go/internal/winapi"
)

type System struct {
	id         uuid.UUID
	handle     windows.Handle
	terminated bool
}

type operation struct {
	handle windows.Handle
}

func CreateComputeSystem(id uuid.UUID, configuration string) (*System, error) {
	op, err := createOperation()
	if err != nil {
		return nil, err
	}
	defer op.Close()

	idPtr, err := winapi.UTF16Ptr(id.String())
	if err != nil {
		return nil, err
	}
	configPtr, err := winapi.UTF16Ptr(configuration)
	if err != nil {
		return nil, err
	}

	var handle windows.Handle
	hr, _, _ := winapi.ProcHcsCreateComputeSystem.Call(
		uintptr(unsafe.Pointer(idPtr)),
		uintptr(unsafe.Pointer(configPtr)),
		uintptr(op.handle),
		0,
		uintptr(unsafe.Pointer(&handle)),
	)
	if winapi.Failed(hr) {
		if handle != 0 {
			winapi.ProcHcsCloseComputeSystem.Call(uintptr(handle))
		}
		return nil, winapi.HRESULTError("HcsCreateComputeSystem", hr, "")
	}
	if _, err := op.Wait("HcsCreateComputeSystem"); err != nil {
		if handle != 0 {
			winapi.ProcHcsCloseComputeSystem.Call(uintptr(handle))
		}
		return nil, err
	}
	return &System{id: id, handle: handle}, nil
}

func (s *System) ID() uuid.UUID { return s.id }

func (s *System) Start() error {
	op, err := createOperation()
	if err != nil {
		return err
	}
	defer op.Close()
	hr, _, _ := winapi.ProcHcsStartComputeSystem.Call(uintptr(s.handle), uintptr(op.handle), 0)
	if winapi.Failed(hr) {
		return winapi.HRESULTError("HcsStartComputeSystem", hr, "")
	}
	_, err = op.Wait("HcsStartComputeSystem")
	return err
}

func (s *System) Modify(configuration string) error {
	op, err := createOperation()
	if err != nil {
		return err
	}
	defer op.Close()
	configPtr, err := winapi.UTF16Ptr(configuration)
	if err != nil {
		return err
	}
	hr, _, _ := winapi.ProcHcsModifyComputeSystem.Call(uintptr(s.handle), uintptr(op.handle), uintptr(unsafe.Pointer(configPtr)), 0)
	if winapi.Failed(hr) {
		return winapi.HRESULTError("HcsModifyComputeSystem", hr, "")
	}
	_, err = op.Wait("HcsModifyComputeSystem")
	return err
}

func (s *System) GetProperties(query string) (string, error) {
	op, err := createOperation()
	if err != nil {
		return "", err
	}
	defer op.Close()
	queryPtr, err := winapi.OptionalUTF16Ptr(query)
	if err != nil {
		return "", err
	}
	hr, _, _ := winapi.ProcHcsGetComputeSystemProperties.Call(uintptr(s.handle), uintptr(op.handle), queryPtr)
	if winapi.Failed(hr) {
		return "", winapi.HRESULTError("HcsGetComputeSystemProperties", hr, "")
	}
	return op.Wait("HcsGetComputeSystemProperties")
}

func (s *System) Terminate() error {
	if s.terminated || s.handle == 0 {
		return nil
	}
	op, err := createOperation()
	if err != nil {
		return err
	}
	defer op.Close()
	hr, _, _ := winapi.ProcHcsTerminateComputeSystem.Call(uintptr(s.handle), uintptr(op.handle), 0)
	if winapi.Failed(hr) {
		return winapi.HRESULTError("HcsTerminateComputeSystem", hr, "")
	}
	if _, err := op.Wait("HcsTerminateComputeSystem"); err != nil {
		return err
	}
	s.terminated = true
	return nil
}

func (s *System) Close() error {
	if s.handle != 0 {
		winapi.ProcHcsCloseComputeSystem.Call(uintptr(s.handle))
		s.handle = 0
		runtime.KeepAlive(s)
	}
	return nil
}

func createOperation() (*operation, error) {
	r1, _, err := winapi.ProcHcsCreateOperation.Call(0, 0)
	if r1 == 0 {
		return nil, err
	}
	return &operation{handle: windows.Handle(r1)}, nil
}

func (o *operation) Wait(name string) (string, error) {
	var result uintptr
	hr, _, _ := winapi.ProcHcsWaitForOperationResult.Call(uintptr(o.handle), winapi.Infinite, uintptr(unsafe.Pointer(&result)))
	doc := winapi.ConsumeNativeString(result)
	if winapi.Failed(hr) {
		return "", winapi.HRESULTError(name, hr, doc)
	}
	return doc, nil
}

func (o *operation) Close() error {
	if o.handle != 0 {
		winapi.ProcHcsCloseOperation.Call(uintptr(o.handle))
		o.handle = 0
	}
	return nil
}
