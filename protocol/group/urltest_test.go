package group

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

func TestURLTestGroupSelectSkipsUnavailableHistory(t *testing.T) {
	historyStorage := urltest.NewHistoryStorage()
	historyStorage.StoreURLTestHistory("dead", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  5,
		Status: adapter.URLTestStatusUnavailable,
		Error:  "timeout",
	})
	historyStorage.StoreURLTestHistory("alive", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  120,
		Status: adapter.URLTestStatusAvailable,
	})
	historyStorage.StoreURLTestHistory("unknown", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  0,
		Status: adapter.URLTestStatusAvailable,
	})
	group := &URLTestGroup{
		outbounds: []adapter.Outbound{
			&testOutbound{tag: "dead"},
			&testOutbound{tag: "unknown"},
			&testOutbound{tag: "alive"},
		},
		history:   historyStorage,
		tolerance: 50,
	}

	selected, exists := group.Select(N.NetworkTCP)

	require.True(t, exists)
	require.NotNil(t, selected)
	require.Equal(t, "alive", selected.Tag())
}

func TestURLTestGroupCurrentOutboundFailoverOnUnavailableSelection(t *testing.T) {
	historyStorage := urltest.NewHistoryStorage()
	historyStorage.StoreURLTestHistory("dead", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  5,
		Status: adapter.URLTestStatusUnavailable,
		Error:  "timeout",
	})
	historyStorage.StoreURLTestHistory("alive", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  120,
		Status: adapter.URLTestStatusAvailable,
	})
	dead := &testOutbound{tag: "dead"}
	alive := &testOutbound{tag: "alive"}
	group := &URLTestGroup{
		outbounds:           []adapter.Outbound{dead, alive},
		history:             historyStorage,
		tolerance:           50,
		selectedOutboundTCP: dead,
	}

	selected := group.currentOutbound(N.NetworkTCP)

	require.NotNil(t, selected)
	require.Equal(t, "alive", selected.Tag())
	require.Equal(t, alive, group.selectedOutboundTCP)
}

func TestURLTestGroupReportFailureSwitchesToAlternateOutbound(t *testing.T) {
	historyStorage := urltest.NewHistoryStorage()
	historyStorage.StoreURLTestHistory("dead", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  5,
		Status: adapter.URLTestStatusAvailable,
	})
	historyStorage.StoreURLTestHistory("alive", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  120,
		Status: adapter.URLTestStatusAvailable,
	})
	dead := &testOutbound{tag: "dead", networks: []string{N.NetworkTCP, N.NetworkUDP}}
	alive := &testOutbound{tag: "alive", networks: []string{N.NetworkTCP, N.NetworkUDP}}
	group := &URLTestGroup{
		ctx:                 service.ContextWith[adapter.URLTestHistoryStorage](context.Background(), historyStorage),
		outbounds:           []adapter.Outbound{dead, alive},
		history:             historyStorage,
		tolerance:           50,
		selectedOutboundTCP: dead,
		selectedOutboundUDP: dead,
	}

	selected := group.reportFailure(dead, context.DeadlineExceeded, N.NetworkTCP)

	require.NotNil(t, selected)
	require.Equal(t, "alive", selected.Tag())
	require.Equal(t, alive, group.selectedOutboundTCP)
	require.Equal(t, dead, group.selectedOutboundUDP)
	history := historyStorage.LoadURLTestHistory("dead")
	require.NotNil(t, history)
	require.Equal(t, adapter.URLTestStatusUnavailable, history.Status)
	require.Equal(t, context.DeadlineExceeded.Error(), history.Error)
}

func TestURLTestGroupCurrentOutboundSwitchesOtherNetworkAfterEndpointFailure(t *testing.T) {
	historyStorage := urltest.NewHistoryStorage()
	historyStorage.StoreURLTestHistory("dead", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  5,
		Status: adapter.URLTestStatusAvailable,
	})
	historyStorage.StoreURLTestHistory("alive", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  120,
		Status: adapter.URLTestStatusAvailable,
	})
	dead := &testOutbound{tag: "dead", networks: []string{N.NetworkTCP, N.NetworkUDP}}
	alive := &testOutbound{tag: "alive", networks: []string{N.NetworkTCP, N.NetworkUDP}}
	group := &URLTestGroup{
		ctx:                 service.ContextWith[adapter.URLTestHistoryStorage](context.Background(), historyStorage),
		outbounds:           []adapter.Outbound{dead, alive},
		history:             historyStorage,
		tolerance:           50,
		selectedOutboundTCP: dead,
		selectedOutboundUDP: dead,
	}

	group.reportFailure(dead, context.DeadlineExceeded, N.NetworkTCP)
	selectedUDP := group.currentOutbound(N.NetworkUDP)

	require.NotNil(t, selectedUDP)
	require.Equal(t, "alive", selectedUDP.Tag())
	require.Equal(t, alive, group.selectedOutboundUDP)
}

func TestURLTestGroupRecheckUnavailableOutboundRecoversSpecificOutbound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	historyStorage := urltest.NewHistoryStorage()
	historyStorage.StoreURLTestHistory("dead", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  5,
		Status: adapter.URLTestStatusUnavailable,
		Error:  "timeout",
	})
	historyStorage.StoreURLTestHistory("alive", &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  120,
		Status: adapter.URLTestStatusAvailable,
	})
	dead := &testOutbound{tag: "dead", networks: []string{N.NetworkTCP}, dialDestination: true}
	alive := &testOutbound{tag: "alive", networks: []string{N.NetworkTCP}, dialDestination: true}
	manager := &testOutboundManager{
		outbounds: map[string]adapter.Outbound{
			"dead":  dead,
			"alive": alive,
		},
	}
	group := &URLTestGroup{
		ctx:                 context.Background(),
		outbound:            manager,
		logger:              log.NewNOPFactory().Logger(),
		outbounds:           []adapter.Outbound{dead, alive},
		link:                server.URL,
		history:             historyStorage,
		tolerance:           50,
		interruptGroup:      interrupt.NewGroup(),
		selectedOutboundTCP: alive,
	}

	recovered := group.recheckUnavailableOutbound("dead")

	if !recovered {
		t.Fatalf("recheck failed: %+v", historyStorage.LoadURLTestHistory("dead"))
	}
	history := historyStorage.LoadURLTestHistory("dead")
	require.NotNil(t, history)
	require.Equal(t, adapter.URLTestStatusAvailable, history.Status)
	require.NotZero(t, history.Delay)
	require.Equal(t, "dead", group.currentOutbound(N.NetworkTCP).Tag())
}

type testOutbound struct {
	tag             string
	networks        []string
	dialDestination bool
}

func (o *testOutbound) Type() string { return "test" }
func (o *testOutbound) Tag() string  { return o.tag }
func (o *testOutbound) Network() []string {
	if len(o.networks) == 0 {
		return []string{N.NetworkTCP}
	}
	return o.networks
}
func (o *testOutbound) Dependencies() []string { return nil }
func (o *testOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if o.dialDestination {
		return new(net.Dialer).DialContext(ctx, network, destination.String())
	}
	return nil, nil
}
func (o *testOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

var _ adapter.Outbound = (*testOutbound)(nil)

type testOutboundManager struct {
	outbounds map[string]adapter.Outbound
}

func (m *testOutboundManager) Start(stage adapter.StartStage) error { return nil }
func (m *testOutboundManager) Close() error                         { return nil }
func (m *testOutboundManager) Outbounds() []adapter.Outbound {
	var outbounds []adapter.Outbound
	for _, outbound := range m.outbounds {
		outbounds = append(outbounds, outbound)
	}
	return outbounds
}
func (m *testOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := m.outbounds[tag]
	return outbound, loaded
}
func (m *testOutboundManager) Default() adapter.Outbound { return nil }
func (m *testOutboundManager) Remove(tag string) error   { return nil }
func (m *testOutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return nil
}

var _ adapter.OutboundManager = (*testOutboundManager)(nil)
