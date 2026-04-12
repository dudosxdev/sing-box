package adapter

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

func TestStoreURLTestFailurePreservesDelay(t *testing.T) {
	historyStorage := newTestURLTestHistoryStorage()
	historyStorage.StoreURLTestHistory("node-a", &URLTestHistory{
		Time:   time.Now().Add(-time.Minute),
		Delay:  321,
		Status: URLTestStatusAvailable,
	})
	ctx := service.ContextWith[URLTestHistoryStorage](context.Background(), historyStorage)

	StoreURLTestFailure(ctx, &testOutbound{tag: "node-a"}, errors.New("i/o timeout"))

	history := historyStorage.LoadURLTestHistory("node-a")
	require.NotNil(t, history)
	require.Equal(t, uint16(321), history.Delay)
	require.Equal(t, URLTestStatusUnavailable, history.Status)
	require.Equal(t, "i/o timeout", history.Error)
}

func TestStoreURLTestFailureUsesCurrentGroupSelection(t *testing.T) {
	historyStorage := newTestURLTestHistoryStorage()
	ctx := service.ContextWith[URLTestHistoryStorage](context.Background(), historyStorage)

	StoreURLTestFailure(ctx, &testOutboundGroup{testOutbound: testOutbound{tag: "group-a"}, now: "node-b"}, errors.New("connection refused"))

	history := historyStorage.LoadURLTestHistory("node-b")
	require.NotNil(t, history)
	require.Equal(t, URLTestStatusUnavailable, history.Status)
	require.Equal(t, "connection refused", history.Error)
}

type testURLTestHistoryStorage struct {
	access sync.Mutex
	store  map[string]*URLTestHistory
}

func newTestURLTestHistoryStorage() *testURLTestHistoryStorage {
	return &testURLTestHistoryStorage{
		store: make(map[string]*URLTestHistory),
	}
}

func (s *testURLTestHistoryStorage) SetHook(hook *observable.Subscriber[struct{}]) {}

func (s *testURLTestHistoryStorage) LoadURLTestHistory(tag string) *URLTestHistory {
	s.access.Lock()
	defer s.access.Unlock()
	history := s.store[tag]
	if history == nil {
		return nil
	}
	copy := *history
	return &copy
}

func (s *testURLTestHistoryStorage) DeleteURLTestHistory(tag string) {
	s.access.Lock()
	defer s.access.Unlock()
	delete(s.store, tag)
}

func (s *testURLTestHistoryStorage) StoreURLTestHistory(tag string, history *URLTestHistory) {
	s.access.Lock()
	defer s.access.Unlock()
	copy := *history
	s.store[tag] = &copy
}

func (s *testURLTestHistoryStorage) Close() error {
	return nil
}

type testOutbound struct {
	tag string
}

func (o *testOutbound) Type() string           { return "test" }
func (o *testOutbound) Tag() string            { return o.tag }
func (o *testOutbound) Network() []string      { return []string{"tcp"} }
func (o *testOutbound) Dependencies() []string { return nil }
func (o *testOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (o *testOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

type testOutboundGroup struct {
	testOutbound
	now string
}

func (o *testOutboundGroup) Now() string   { return o.now }
func (o *testOutboundGroup) All() []string { return []string{o.now} }

var _ Outbound = (*testOutbound)(nil)
var _ OutboundGroup = (*testOutboundGroup)(nil)
