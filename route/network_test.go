package route

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service/pause"
	"github.com/stretchr/testify/require"
)

func TestNotifyInterfaceUpdateSkipsCloseAllOnAndroidVPNUpdate(t *testing.T) {
	connectionManager := &testConnectionManager{}
	outboundListener := &testOutboundListener{}
	manager := &NetworkManager{
		logger:               log.NewNOPFactory().Logger(),
		connectionManager:    connectionManager,
		endpoint:             testEndpointManager{},
		inbound:              testInboundManager{},
		outbound:             testOutboundManager{outbounds: []adapter.Outbound{outboundListener}},
		pauseManager:         &testPauseManager{},
		started:              true,
		lastDefaultInterface: cloneInterface(testControlInterface()),
	}

	manager.notifyInterfaceUpdate(testControlInterface(), tun.FlagAndroidVPNUpdate)

	require.Zero(t, connectionManager.closeAllCalls)
	require.Equal(t, 1, outboundListener.interfaceUpdates)
}

func TestNotifyInterfaceUpdateClosesConnectionsOnInterfaceChange(t *testing.T) {
	connectionManager := &testConnectionManager{}
	manager := &NetworkManager{
		logger:               log.NewNOPFactory().Logger(),
		connectionManager:    connectionManager,
		endpoint:             testEndpointManager{},
		inbound:              testInboundManager{},
		outbound:             testOutboundManager{},
		pauseManager:         &testPauseManager{},
		started:              true,
		lastDefaultInterface: cloneInterface(testControlInterface()),
	}

	updatedInterface := testControlInterface()
	updatedInterface.Index = 2
	updatedInterface.Name = "rmnet_data1"

	manager.notifyInterfaceUpdate(updatedInterface, tun.FlagAndroidVPNUpdate)

	require.Equal(t, 1, connectionManager.closeAllCalls)
}

func testControlInterface() *control.Interface {
	return &control.Interface{
		Index: 1,
		MTU:   1500,
		Name:  "rmnet_data0",
		Addresses: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.2/24"),
		},
	}
}

type testConnectionManager struct {
	closeAllCalls int
}

func (m *testConnectionManager) Start(stage adapter.StartStage) error { return nil }
func (m *testConnectionManager) Close() error                         { return nil }
func (m *testConnectionManager) Count() int                           { return 0 }
func (m *testConnectionManager) CloseAll()                            { m.closeAllCalls++ }
func (m *testConnectionManager) TrackConn(conn net.Conn) net.Conn     { return conn }
func (m *testConnectionManager) TrackPacketConn(conn net.PacketConn) net.PacketConn {
	return conn
}
func (m *testConnectionManager) NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
}
func (m *testConnectionManager) NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
}

type testEndpointManager struct{}

func (m testEndpointManager) Start(stage adapter.StartStage) error { return nil }
func (m testEndpointManager) Close() error                         { return nil }
func (m testEndpointManager) Endpoints() []adapter.Endpoint        { return nil }
func (m testEndpointManager) Get(tag string) (adapter.Endpoint, bool) {
	return nil, false
}
func (m testEndpointManager) Remove(tag string) error {
	return nil
}
func (m testEndpointManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, endpointType string, options any) error {
	return nil
}

type testInboundManager struct{}

func (m testInboundManager) Start(stage adapter.StartStage) error { return nil }
func (m testInboundManager) Close() error                         { return nil }
func (m testInboundManager) Inbounds() []adapter.Inbound          { return nil }
func (m testInboundManager) Get(tag string) (adapter.Inbound, bool) {
	return nil, false
}
func (m testInboundManager) Remove(tag string) error {
	return nil
}
func (m testInboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, inboundType string, options any) error {
	return nil
}

type testOutboundManager struct {
	outbounds []adapter.Outbound
}

func (m testOutboundManager) Start(stage adapter.StartStage) error { return nil }
func (m testOutboundManager) Close() error                         { return nil }
func (m testOutboundManager) Outbounds() []adapter.Outbound        { return m.outbounds }
func (m testOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	for _, outbound := range m.outbounds {
		if outbound.Tag() == tag {
			return outbound, true
		}
	}
	return nil, false
}
func (m testOutboundManager) Default() adapter.Outbound {
	if len(m.outbounds) == 0 {
		return nil
	}
	return m.outbounds[0]
}
func (m testOutboundManager) Remove(tag string) error {
	return nil
}
func (m testOutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return nil
}

type testOutboundListener struct {
	interfaceUpdates int
}

func (o *testOutboundListener) Type() string           { return "direct" }
func (o *testOutboundListener) Tag() string            { return "test" }
func (o *testOutboundListener) Network() []string      { return nil }
func (o *testOutboundListener) Dependencies() []string { return nil }
func (o *testOutboundListener) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (o *testOutboundListener) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}
func (o *testOutboundListener) InterfaceUpdated() {
	o.interfaceUpdates++
}

type testPauseManager struct{}

func (m *testPauseManager) DevicePause()          {}
func (m *testPauseManager) DeviceWake()           {}
func (m *testPauseManager) NetworkPause()         {}
func (m *testPauseManager) NetworkWake()          {}
func (m *testPauseManager) IsDevicePaused() bool  { return false }
func (m *testPauseManager) IsNetworkPaused() bool { return false }
func (m *testPauseManager) IsPaused() bool        { return false }
func (m *testPauseManager) WaitActive()           {}
func (m *testPauseManager) RegisterCallback(callback pause.Callback) *list.Element[pause.Callback] {
	return nil
}
func (m *testPauseManager) UnregisterCallback(element *list.Element[pause.Callback]) {}

var _ adapter.ConnectionManager = (*testConnectionManager)(nil)
var _ adapter.EndpointManager = (*testEndpointManager)(nil)
var _ adapter.InboundManager = (*testInboundManager)(nil)
var _ adapter.OutboundManager = (*testOutboundManager)(nil)
var _ adapter.Outbound = (*testOutboundListener)(nil)
var _ pause.Manager = (*testPauseManager)(nil)
