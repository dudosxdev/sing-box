package kcp

// Notifier is a simple channel-based notifier for signaling changes.
type Notifier struct {
	c chan struct{}
}

// NewNotifier creates a new Notifier.
func NewNotifier() *Notifier {
	return &Notifier{
		c: make(chan struct{}, 1),
	}
}

// Signal signals a change. This method never blocks.
func (n *Notifier) Signal() {
	select {
	case n.c <- struct{}{}:
	default:
	}
}

// Wait returns a channel for waiting for changes.
func (n *Notifier) Wait() <-chan struct{} {
	return n.c
}
