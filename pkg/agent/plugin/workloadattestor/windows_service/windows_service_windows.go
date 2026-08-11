//go:build windows

package windows_service

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/hashicorp/go-hclog"
	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	"github.com/spiffe/spire/pkg/common/catalog"
	"github.com/spiffe/spire/pkg/common/util"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func builtin(p *Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		workloadattestorv1.WorkloadAttestorPluginServer(p),
	)
}

type ServiceInfo struct {
	Name        string
	DisplayName string
}

type Plugin struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer

	log hclog.Logger

	// hook for tests
	getServiceInfo func(pid uint32) (*ServiceInfo, error)
}

func New() *Plugin {
	p := &Plugin{}
	p.getServiceInfo = LookupServiceName
	return p
}

func (p *Plugin) SetLogger(log hclog.Logger) {
	p.log = log
}

// AttestReference returns Unimplemented. This plugin does not handle
// reference-based workload attestation; the host falls back to PID-based
// Attest when the reference is a WorkloadPIDReference.
func (p *Plugin) AttestReference(_ context.Context, _ *workloadattestorv1.AttestReferenceRequest) (*workloadattestorv1.AttestReferenceResponse, error) {
	return nil, status.Error(codes.Unimplemented, "AttestReference not implemented")
}

func (p *Plugin) Attest(_ context.Context, req *workloadattestorv1.AttestRequest) (*workloadattestorv1.AttestResponse, error) {
	pid, err := util.CheckedCast[uint32](req.Pid)
	if err != nil {
		return nil, fmt.Errorf("invalid value for PID: %w", err)
	}

	serviceInfo, err := p.getServiceInfo(pid)
	if err != nil {
		if p.log != nil {
			p.log.Warn("Failed to determine whether process belongs to a Windows service", "pid", req.Pid, "error", err)
		}
		return nil, err
	}

	var selectorValues []string

	if serviceInfo != nil {
		selectorValues = append(selectorValues, makeSelectorValue("name", serviceInfo.Name))
		selectorValues = append(selectorValues, makeSelectorValue("display_name", serviceInfo.DisplayName))
	}

	return &workloadattestorv1.AttestResponse{
		SelectorValues: selectorValues,
	}, nil
}

func makeSelectorValue(kind, value string) string {
	return fmt.Sprintf("%s:%s", kind, value)
}

func LookupServiceName(pid uint32) (*ServiceInfo, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, fmt.Errorf("failed to open service control manager: %w", err)
	}
	defer func(handle windows.Handle) {
		_ = windows.CloseServiceHandle(handle)
	}(scm)

	services, err := enumServicesStatusProcess(scm)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate services: %w", err)
	}

	for _, svc := range services {
		if svc.ServiceStatusProcess.ProcessId == pid {
			name := windows.UTF16PtrToString(svc.ServiceName)
			displayName := windows.UTF16PtrToString(svc.DisplayName)
			return &ServiceInfo{Name: name, DisplayName: displayName}, nil
		}
	}

	return nil, nil
}

// enumServicesStatusProcess enumerates all Win32 services (in any state)
// registered with the given SCM handle, returning their status and owning
// process id.
func enumServicesStatusProcess(scm windows.Handle) ([]windows.ENUM_SERVICE_STATUS_PROCESS, error) {
	var (
		bytesNeeded      uint32
		servicesReturned uint32
		resumeHandle     uint32
		buf              []byte
	)

	for {
		var p *byte
		if len(buf) > 0 {
			p = &buf[0]
		}
		bufLen, _ := util.CheckedCast[uint32](len(buf))

		err := windows.EnumServicesStatusEx(
			scm,
			windows.SC_ENUM_PROCESS_INFO,
			windows.SERVICE_WIN32,
			windows.SERVICE_STATE_ALL,
			p,
			bufLen,
			&bytesNeeded,
			&servicesReturned,
			&resumeHandle,
			nil,
		)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ERROR_MORE_DATA) {
			return nil, err
		}
		buf = make([]byte, bytesNeeded)
	}

	if servicesReturned == 0 {
		return nil, nil
	}

	entries := unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0])), int(servicesReturned))
	result := make([]windows.ENUM_SERVICE_STATUS_PROCESS, len(entries))
	copy(result, entries)
	return result, nil
}
