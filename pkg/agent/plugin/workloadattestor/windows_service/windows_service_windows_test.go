//go:build windows

package windows_service

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spiffe/spire/pkg/agent/plugin/workloadattestor"
	"github.com/spiffe/spire/test/plugintest"
	"github.com/spiffe/spire/test/spiretest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ctx = context.Background()
)

func TestPlugin(t *testing.T) {
	testCases := []struct {
		name           string
		pid            int
		selectorValues []string
		expectCode     codes.Code
		expectMsg      string
		expectLogs     []spiretest.LogEntry
	}{
		{
			name:           "get service info",
			pid:            1,
			expectCode:     codes.OK,
			selectorValues: []string{"name:fake.service", "display_name:Really Fake Service Description"},
		},
		{
			name:       "fail to get service info",
			pid:        2,
			expectCode: codes.Internal,
			expectMsg:  "workloadattestor(windows_service): failed to open service control manager",
		},
		{
			name:           "caller is not a service",
			pid:            3,
			expectCode:     codes.OK,
			selectorValues: nil,
		},
	}

	for _, testCase := range testCases {
		log, logHook := test.NewNullLogger()
		t.Run(testCase.name, func(t *testing.T) {
			p := loadPlugin(t, log)
			selectors, err := p.Attest(ctx, testCase.pid)
			spiretest.RequireGRPCStatus(t, err, testCase.expectCode, testCase.expectMsg)
			if testCase.expectCode != codes.OK {
				require.Nil(t, selectors)
				return
			}

			require.NoError(t, err)
			var selectorValues []string
			for _, selector := range selectors {
				require.Equal(t, "windows_service", selector.Type)
				selectorValues = append(selectorValues, selector.Value)
			}

			require.Equal(t, testCase.selectorValues, selectorValues)
			spiretest.AssertLogs(t, logHook.AllEntries(), testCase.expectLogs)
		})
	}
}

func loadPlugin(t *testing.T, log logrus.FieldLogger) workloadattestor.WorkloadAttestor {
	p := newPlugin()

	v1 := new(workloadattestor.V1)
	plugintest.Load(t, builtin(p), v1, plugintest.Log(log))
	return v1
}

func newPlugin() *Plugin {
	p := New()
	p.getServiceInfo = func(pid uint32) (*ServiceInfo, error) {
		switch pid {
		case 1:
			return &ServiceInfo{"fake.service", "Really Fake Service Description"}, nil
		case 2:
			return nil, status.Errorf(codes.Internal, "failed to open service control manager")
		case 3:
			return nil, nil
		default:
			return nil, status.Errorf(codes.Internal, "unhandled test case %d", pid)
		}
	}
	return p
}
