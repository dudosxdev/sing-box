package group

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

func RegisterURLTest(registry *outbound.Registry) {
	outbound.Register[option.URLTestOutboundOptions](registry, C.TypeURLTest, NewURLTest)
}

var _ adapter.OutboundGroup = (*URLTest)(nil)

type URLTest struct {
	outbound.Adapter
	ctx                          context.Context
	router                       adapter.Router
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tags                         []string
	link                         string
	interval                     time.Duration
	unavailableCheckInterval     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	group                        *URLTestGroup
	interruptExternalConnections bool
}

func NewURLTest(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.URLTestOutboundOptions) (adapter.Outbound, error) {
	outbound := &URLTest{
		Adapter:                      outbound.NewAdapter(C.TypeURLTest, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:                          ctx,
		router:                       router,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         options.Outbounds,
		link:                         options.URL,
		interval:                     time.Duration(options.Interval),
		unavailableCheckInterval:     time.Duration(options.UnavailableCheckInterval),
		tolerance:                    options.Tolerance,
		idleTimeout:                  time.Duration(options.IdleTimeout),
		interruptExternalConnections: options.InterruptExistConnections,
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (s *URLTest) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(s.tags))
	for i, tag := range s.tags {
		detour, loaded := s.outbound.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		outbounds = append(outbounds, detour)
	}
	group, err := NewURLTestGroup(s.ctx, s.outbound, s.logger, outbounds, s.link, s.interval, s.unavailableCheckInterval, s.tolerance, s.idleTimeout, s.interruptExternalConnections)
	if err != nil {
		return err
	}
	s.group = group
	return nil
}

func (s *URLTest) PostStart() error {
	s.group.PostStart()
	return nil
}

func (s *URLTest) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *URLTest) Now() string {
	if outbound := s.group.selectedOutbound(N.NetworkTCP); outbound != nil {
		return outbound.Tag()
	} else if outbound := s.group.selectedOutbound(N.NetworkUDP); outbound != nil {
		return outbound.Tag()
	}
	return ""
}

func (s *URLTest) All() []string {
	return s.tags
}

func (s *URLTest) URLTest(ctx context.Context) (map[string]uint16, error) {
	return s.group.URLTest(ctx)
}

func (s *URLTest) CheckOutbounds() {
	s.group.CheckOutbounds(true)
}

func (s *URLTest) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	s.group.Touch()
	networkName := N.NetworkName(network)
	switch networkName {
	case N.NetworkTCP, N.NetworkUDP:
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	outbound := s.group.currentOutbound(networkName)
	if outbound == nil {
		return nil, E.New("missing supported outbound")
	}
	conn, err := outbound.DialContext(ctx, network, destination)
	if err == nil {
		return s.group.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	s.logger.ErrorContext(ctx, err)
	retryOutbound := s.group.reportFailure(outbound, err, networkName)
	if retryOutbound == nil || retryOutbound == outbound {
		return nil, err
	}
	conn, retryErr := retryOutbound.DialContext(ctx, network, destination)
	if retryErr != nil {
		s.logger.ErrorContext(ctx, retryErr)
		s.group.reportFailure(retryOutbound, retryErr, networkName)
		return nil, retryErr
	}
	return s.group.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (s *URLTest) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	s.group.Touch()
	outbound := s.group.currentOutbound(N.NetworkUDP)
	if outbound == nil {
		return nil, E.New("missing supported outbound")
	}
	conn, err := outbound.ListenPacket(ctx, destination)
	if err == nil {
		return s.group.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	s.logger.ErrorContext(ctx, err)
	retryOutbound := s.group.reportFailure(outbound, err, N.NetworkUDP)
	if retryOutbound == nil || retryOutbound == outbound {
		return nil, err
	}
	conn, retryErr := retryOutbound.ListenPacket(ctx, destination)
	if retryErr != nil {
		s.logger.ErrorContext(ctx, retryErr)
		s.group.reportFailure(retryOutbound, retryErr, N.NetworkUDP)
		return nil, retryErr
	}
	return s.group.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (s *URLTest) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *URLTest) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

func (s *URLTest) NewDirectRouteConnection(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	s.group.Touch()
	selected := s.group.currentOutbound(N.NetworkTCP)
	if selected == nil {
		return nil, E.New("missing supported outbound")
	}
	if !common.Contains(selected.Network(), metadata.Network) {
		return nil, E.New(metadata.Network, " is not supported by outbound: ", selected.Tag())
	}
	return selected.(adapter.DirectRouteOutbound).NewDirectRouteConnection(metadata, routeContext, timeout)
}

type URLTestGroup struct {
	ctx                          context.Context
	router                       adapter.Router
	outbound                     adapter.OutboundManager
	pause                        pause.Manager
	pauseCallback                *list.Element[pause.Callback]
	logger                       log.Logger
	outbounds                    []adapter.Outbound
	link                         string
	interval                     time.Duration
	unavailableCheckInterval     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	history                      adapter.URLTestHistoryStorage
	checking                     atomic.Bool
	selectedOutboundTCP          adapter.Outbound
	selectedOutboundUDP          adapter.Outbound
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
	access                       sync.Mutex
	ticker                       *time.Ticker
	close                        chan struct{}
	closeOnce                    sync.Once
	started                      bool
	lastActive                   common.TypedValue[time.Time]
	recheckAccess                sync.Mutex
	unavailableRecheck           map[string]chan struct{}
}

func NewURLTestGroup(ctx context.Context, outboundManager adapter.OutboundManager, logger log.Logger, outbounds []adapter.Outbound, link string, interval time.Duration, unavailableCheckInterval time.Duration, tolerance uint16, idleTimeout time.Duration, interruptExternalConnections bool) (*URLTestGroup, error) {
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if unavailableCheckInterval == 0 {
		unavailableCheckInterval = C.DefaultURLTestUnavailableCheckInterval
	}
	if tolerance == 0 {
		tolerance = 50
	}
	if idleTimeout == 0 {
		idleTimeout = C.DefaultURLTestIdleTimeout
	}
	if interval > idleTimeout {
		return nil, E.New("interval must be less or equal than idle_timeout")
	}
	var history adapter.URLTestHistoryStorage
	if historyFromCtx := service.PtrFromContext[urltest.HistoryStorage](ctx); historyFromCtx != nil {
		history = historyFromCtx
	} else if clashServer := service.FromContext[adapter.ClashServer](ctx); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	return &URLTestGroup{
		ctx:                          ctx,
		outbound:                     outboundManager,
		logger:                       logger,
		outbounds:                    outbounds,
		link:                         link,
		interval:                     interval,
		unavailableCheckInterval:     unavailableCheckInterval,
		tolerance:                    tolerance,
		idleTimeout:                  idleTimeout,
		history:                      history,
		close:                        make(chan struct{}),
		unavailableRecheck:           make(map[string]chan struct{}),
		pause:                        service.FromContext[pause.Manager](ctx),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: interruptExternalConnections,
	}, nil
}

func (g *URLTestGroup) PostStart() {
	g.access.Lock()
	defer g.access.Unlock()
	g.started = true
	g.lastActive.Store(time.Now())
	go g.CheckOutbounds(false)
}

func (g *URLTestGroup) Touch() {
	if !g.started {
		return
	}
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker != nil {
		g.lastActive.Store(time.Now())
		return
	}
	g.ticker = time.NewTicker(g.interval)
	go g.loopCheck()
	g.pauseCallback = pause.RegisterTicker(g.pause, g.ticker, g.interval, nil)
}

func (g *URLTestGroup) Close() error {
	g.access.Lock()
	if g.ticker != nil {
		g.ticker.Stop()
		g.pause.UnregisterCallback(g.pauseCallback)
		g.pauseCallback = nil
	}
	g.ticker = nil
	g.access.Unlock()
	g.closeOnce.Do(func() {
		close(g.close)
	})
	return nil
}

func (g *URLTestGroup) Select(network string) (adapter.Outbound, bool) {
	return g.selectOutbound(network, nil)
}

func (g *URLTestGroup) selectOutbound(network string, failed adapter.Outbound) (adapter.Outbound, bool) {
	var minDelay uint16
	var minOutbound adapter.Outbound
	failedTag := ""
	if failed != nil {
		failedTag = RealTag(failed)
	}
	switch network {
	case N.NetworkTCP:
		if selected := g.selectedOutbound(N.NetworkTCP); selected != nil {
			if RealTag(selected) != failedTag {
				if history := g.history.LoadURLTestHistory(RealTag(selected)); history != nil && history.Status != adapter.URLTestStatusUnavailable && history.Delay > 0 {
					minOutbound = selected
					minDelay = history.Delay
				}
			}
		}
	case N.NetworkUDP:
		if selected := g.selectedOutbound(N.NetworkUDP); selected != nil {
			if RealTag(selected) != failedTag {
				if history := g.history.LoadURLTestHistory(RealTag(selected)); history != nil && history.Status != adapter.URLTestStatusUnavailable && history.Delay > 0 {
					minOutbound = selected
					minDelay = history.Delay
				}
			}
		}
	}
	for _, detour := range g.outbounds {
		if !common.Contains(detour.Network(), network) || RealTag(detour) == failedTag {
			continue
		}
		history := g.history.LoadURLTestHistory(RealTag(detour))
		if history == nil || history.Status == adapter.URLTestStatusUnavailable || history.Delay == 0 {
			continue
		}
		if minDelay == 0 || minDelay > history.Delay+g.tolerance {
			minDelay = history.Delay
			minOutbound = detour
		}
	}
	if minOutbound == nil {
		for _, detour := range g.outbounds {
			if !common.Contains(detour.Network(), network) || RealTag(detour) == failedTag {
				continue
			}
			return detour, false
		}
		if failed != nil && common.Contains(failed.Network(), network) {
			return failed, false
		}
		return nil, false
	}
	return minOutbound, true
}

func (g *URLTestGroup) currentOutbound(network string) adapter.Outbound {
	selected := g.selectedOutbound(network)
	if selected != nil {
		if history := g.history.LoadURLTestHistory(RealTag(selected)); history != nil && history.Status != adapter.URLTestStatusUnavailable && history.Delay > 0 {
			return selected
		}
	}
	next, _ := g.selectOutbound(network, selected)
	g.setSelectedOutbound(network, next)
	return next
}

func (g *URLTestGroup) reportFailure(failed adapter.Outbound, err error, network string) adapter.Outbound {
	g.storeFailureHistory(RealTag(failed), err)
	g.scheduleUnavailableRecheck(RealTag(failed))
	g.refreshSelectedOutbound(network, failed)
	return g.selectedOutbound(network)
}

func (g *URLTestGroup) refreshSelectedOutbound(network string, failed adapter.Outbound) {
	selected := g.selectedOutbound(network)
	if selected == nil || RealTag(selected) != RealTag(failed) {
		return
	}
	next, _ := g.selectOutbound(network, failed)
	g.setSelectedOutbound(network, next)
}

func (g *URLTestGroup) selectedOutbound(network string) adapter.Outbound {
	g.access.Lock()
	defer g.access.Unlock()
	return g.selectedOutboundLocked(network)
}

func (g *URLTestGroup) selectedOutboundLocked(network string) adapter.Outbound {
	switch network {
	case N.NetworkTCP:
		return g.selectedOutboundTCP
	case N.NetworkUDP:
		return g.selectedOutboundUDP
	default:
		return nil
	}
}

func (g *URLTestGroup) setSelectedOutbound(network string, outbound adapter.Outbound) {
	g.access.Lock()
	defer g.access.Unlock()
	g.setSelectedOutboundLocked(network, outbound)
}

func (g *URLTestGroup) setSelectedOutboundLocked(network string, outbound adapter.Outbound) {
	switch network {
	case N.NetworkTCP:
		g.selectedOutboundTCP = outbound
	case N.NetworkUDP:
		g.selectedOutboundUDP = outbound
	}
}

func (g *URLTestGroup) storeFailureHistory(tag string, err error) {
	if tag == "" || err == nil {
		return
	}
	var delay uint16
	if history := g.history.LoadURLTestHistory(tag); history != nil {
		delay = history.Delay
	}
	g.history.StoreURLTestHistory(tag, &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  delay,
		Status: adapter.URLTestStatusUnavailable,
		Error:  err.Error(),
	})
}

func (g *URLTestGroup) storeSuccessHistory(tag string, delay uint16) {
	if tag == "" {
		return
	}
	g.stopUnavailableRecheck(tag)
	g.history.StoreURLTestHistory(tag, &adapter.URLTestHistory{
		Time:   time.Now(),
		Delay:  delay,
		Status: adapter.URLTestStatusAvailable,
	})
}

func (g *URLTestGroup) scheduleUnavailableRecheck(tag string) {
	if tag == "" || g.unavailableCheckInterval <= 0 {
		return
	}
	g.recheckAccess.Lock()
	if _, exists := g.unavailableRecheck[tag]; exists {
		g.recheckAccess.Unlock()
		return
	}
	stop := make(chan struct{})
	g.unavailableRecheck[tag] = stop
	g.recheckAccess.Unlock()
	go g.recheckUnavailableOutboundAfterDelay(tag, stop)
}

func (g *URLTestGroup) stopUnavailableRecheck(tag string) {
	g.recheckAccess.Lock()
	stop, exists := g.unavailableRecheck[tag]
	if exists {
		delete(g.unavailableRecheck, tag)
	}
	g.recheckAccess.Unlock()
	if exists {
		close(stop)
	}
}

func (g *URLTestGroup) clearUnavailableRecheck(tag string, stop chan struct{}) {
	g.recheckAccess.Lock()
	current, exists := g.unavailableRecheck[tag]
	if exists && current == stop {
		delete(g.unavailableRecheck, tag)
	}
	g.recheckAccess.Unlock()
}

func (g *URLTestGroup) recheckUnavailableOutboundAfterDelay(tag string, stop chan struct{}) {
	timer := time.NewTimer(g.unavailableCheckInterval)
	defer timer.Stop()
	defer g.clearUnavailableRecheck(tag, stop)
	select {
	case <-g.close:
		return
	case <-stop:
		return
	case <-timer.C:
	}
	g.recheckUnavailableOutbound(tag)
}

func (g *URLTestGroup) recheckUnavailableOutbound(tag string) bool {
	history := g.history.LoadURLTestHistory(tag)
	if history == nil || history.Status != adapter.URLTestStatusUnavailable {
		return true
	}
	detour, loaded := g.outbound.Outbound(tag)
	if !loaded {
		return true
	}
	testCtx, cancel := context.WithTimeout(g.ctx, C.TCPTimeout)
	defer cancel()
	t, err := urltest.URLTest(testCtx, g.link, detour)
	if err != nil {
		g.logger.Debug("outbound ", tag, " still unavailable: ", err)
		g.storeFailureHistory(tag, err)
		return false
	}
	g.logger.Debug("outbound ", tag, " recovered: ", t, "ms")
	g.storeSuccessHistory(tag, t)
	g.performUpdateCheck()
	return true
}

func (g *URLTestGroup) loopCheck() {
	if time.Since(g.lastActive.Load()) > g.interval {
		g.lastActive.Store(time.Now())
		g.CheckOutbounds(false)
	}
	for {
		select {
		case <-g.close:
			return
		case <-g.ticker.C:
		}
		if time.Since(g.lastActive.Load()) > g.idleTimeout {
			g.access.Lock()
			g.ticker.Stop()
			g.ticker = nil
			g.pause.UnregisterCallback(g.pauseCallback)
			g.pauseCallback = nil
			g.access.Unlock()
			return
		}
		g.CheckOutbounds(false)
	}
}

func (g *URLTestGroup) CheckOutbounds(force bool) {
	_, _ = g.urlTest(g.ctx, force)
}

func (g *URLTestGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, false)
}

func (g *URLTestGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
	result := make(map[string]uint16)
	if g.checking.Swap(true) {
		return result, nil
	}
	defer g.checking.Store(false)
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](10))
	checked := make(map[string]bool)
	var resultAccess sync.Mutex
	for _, detour := range g.outbounds {
		tag := detour.Tag()
		realTag := RealTag(detour)
		if checked[realTag] {
			continue
		}
		history := g.history.LoadURLTestHistory(realTag)
		if !force &&
			history != nil &&
			history.Status != adapter.URLTestStatusUnavailable &&
			time.Since(history.Time) < g.interval {
			continue
		}
		checked[realTag] = true
		p, loaded := g.outbound.Outbound(realTag)
		if !loaded {
			continue
		}
		b.Go(realTag, func() (any, error) {
			testCtx, cancel := context.WithTimeout(g.ctx, C.TCPTimeout)
			defer cancel()
			t, err := urltest.URLTest(testCtx, g.link, p)
			if err != nil {
				g.logger.Debug("outbound ", tag, " unavailable: ", err)
				g.storeFailureHistory(realTag, err)
			} else {
				g.logger.Debug("outbound ", tag, " available: ", t, "ms")
				g.storeSuccessHistory(realTag, t)
				resultAccess.Lock()
				result[tag] = t
				resultAccess.Unlock()
			}
			return nil, nil
		})
	}
	b.Wait()
	g.performUpdateCheck()
	return result, nil
}

func (g *URLTestGroup) performUpdateCheck() {
	var updated bool
	if outbound, exists := g.Select(N.NetworkTCP); outbound != nil {
		current := g.selectedOutbound(N.NetworkTCP)
		if current == nil || (exists && outbound != current) {
			if current != nil {
				updated = true
			}
			g.setSelectedOutbound(N.NetworkTCP, outbound)
		}
	}
	if outbound, exists := g.Select(N.NetworkUDP); outbound != nil {
		current := g.selectedOutbound(N.NetworkUDP)
		if current == nil || (exists && outbound != current) {
			if current != nil {
				updated = true
			}
			g.setSelectedOutbound(N.NetworkUDP, outbound)
		}
	}
	if updated {
		g.interruptGroup.Interrupt(g.interruptExternalConnections)
	}
}
