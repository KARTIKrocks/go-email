package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"time"
)

// Pool-related sentinel errors.
var (
	// ErrPoolClosed is returned when an operation is attempted on a closed pool.
	ErrPoolClosed = errors.New("smtp: connection pool is closed")

	// ErrPoolTimeout is returned when a connection cannot be obtained within the wait timeout.
	ErrPoolTimeout = errors.New("smtp: connection pool wait timeout")
)

// Pool default constants.
const (
	DefaultMaxIdleConns    = 2
	DefaultPoolMaxLifetime = 30 * time.Minute
	DefaultPoolMaxIdleTime = 5 * time.Minute
	DefaultMaxMessages     = 100
	DefaultPoolWaitTimeout = 5 * time.Second
)

// pooledConn wraps an SMTP connection with lifecycle metadata.
type pooledConn struct {
	client    *smtp.Client
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	msgCount  int
}

// smtpPool manages a pool of reusable SMTP connections.
type smtpPool struct {
	// Immutable config (set at creation, never mutated)
	config       SMTPConfig
	logger       Logger
	maxOpen      int
	maxIdleCount int
	maxLife      time.Duration
	maxIdleTime  time.Duration
	maxMsgs      int
	waitTimeout  time.Duration

	// Overridable for testing
	dialFn func(ctx context.Context) (*pooledConn, error)

	// Mutable state (protected by mu)
	mu        sync.Mutex
	idle      []*pooledConn
	numOpen   int
	waitQueue []chan *pooledConn
	closed    bool

	// Cleaner lifecycle
	cleanerCh chan struct{}
}

// newSMTPPool creates a new connection pool with the given config.
func newSMTPPool(config SMTPConfig, logger Logger) *smtpPool {
	maxIdle := config.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = DefaultMaxIdleConns
	}

	maxLife := config.PoolMaxLifetime
	if maxLife == 0 {
		maxLife = DefaultPoolMaxLifetime
	}

	maxIdleTime := config.PoolMaxIdleTime
	if maxIdleTime == 0 {
		maxIdleTime = DefaultPoolMaxIdleTime
	}

	maxMsgs := config.MaxMessages
	if maxMsgs == 0 {
		maxMsgs = DefaultMaxMessages
	}

	waitTimeout := config.PoolWaitTimeout
	if waitTimeout == 0 {
		waitTimeout = DefaultPoolWaitTimeout
	}

	p := &smtpPool{
		config:       config,
		logger:       logger,
		maxOpen:      config.PoolSize,
		maxIdleCount: maxIdle,
		maxLife:      maxLife,
		maxIdleTime:  maxIdleTime,
		maxMsgs:      maxMsgs,
		waitTimeout:  waitTimeout,
		cleanerCh:    make(chan struct{}),
	}
	p.dialFn = p.dial

	go p.cleaner()

	return p
}

// dial creates a new SMTP connection (TCP + optional TLS + optional AUTH).
func (p *smtpPool) dial(ctx context.Context) (*pooledConn, error) {
	addr := fmt.Sprintf("%s:%d", p.config.Host, p.config.Port)

	dialer := &net.Dialer{
		Timeout: p.config.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	client, err := smtp.NewClient(conn, p.config.Host)
	if err != nil {
		conn.Close() //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("create client: %w", err)
	}

	if p.config.UseTLS {
		tlsConfig := &tls.Config{
			ServerName: p.config.Host,
		}
		if tlsErr := client.StartTLS(tlsConfig); tlsErr != nil {
			client.Close() //nolint:errcheck // best-effort cleanup
			conn.Close()   //nolint:errcheck // best-effort cleanup
			return nil, fmt.Errorf("start tls: %w", tlsErr)
		}
	}

	if p.config.Username != "" && p.config.Password != "" {
		auth := smtp.PlainAuth("", p.config.Username, p.config.Password, p.config.Host)
		if authErr := client.Auth(auth); authErr != nil {
			client.Close() //nolint:errcheck // best-effort cleanup
			conn.Close()   //nolint:errcheck // best-effort cleanup
			return nil, fmt.Errorf("auth: %w", authErr)
		}
	}

	now := time.Now()
	return &pooledConn{
		client:    client,
		conn:      conn,
		createdAt: now,
		lastUsed:  now,
	}, nil
}

// get obtains a connection from the pool or dials a new one.
func (p *smtpPool) get(ctx context.Context) (*pooledConn, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// Try to pop an idle connection (LIFO)
	for len(p.idle) > 0 {
		pc := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Check expiry
		if p.isExpired(pc) {
			p.numOpen--
			p.mu.Unlock()
			p.closeConn(pc)
			p.mu.Lock()
			continue
		}

		p.mu.Unlock()

		// Health check outside the lock
		if err := p.healthCheck(pc); err != nil {
			p.logger.Debug("pool health check failed, discarding connection", "error", err)
			p.mu.Lock()
			p.numOpen--
			p.mu.Unlock()
			p.closeConn(pc)
			return p.get(ctx) // retry
		}

		return pc, nil
	}

	// No idle connections — can we open a new one?
	if p.numOpen < p.maxOpen {
		p.numOpen++ // reserve a slot
		p.mu.Unlock()

		pc, err := p.dialFn(ctx)
		if err != nil {
			p.mu.Lock()
			p.numOpen--
			p.wakeWaiter()
			p.mu.Unlock()
			return nil, err
		}
		return pc, nil
	}

	// Pool exhausted — wait for a connection
	ch := make(chan *pooledConn, 1)
	p.waitQueue = append(p.waitQueue, ch)
	p.mu.Unlock()

	timer := time.NewTimer(p.waitTimeout)
	defer timer.Stop()

	select {
	case pc := <-ch:
		if pc == nil {
			return nil, ErrPoolClosed
		}
		// Health check the received connection
		if err := p.healthCheck(pc); err != nil {
			p.logger.Debug("pool health check failed on waited connection", "error", err)
			p.mu.Lock()
			p.numOpen--
			p.wakeWaiter()
			p.mu.Unlock()
			p.closeConn(pc)
			return p.get(ctx) // retry
		}
		return pc, nil

	case <-timer.C:
		p.mu.Lock()
		p.removeWaiter(ch)
		p.mu.Unlock()
		// Drain channel in case a connection arrived just after timeout
		select {
		case pc := <-ch:
			if pc != nil {
				p.put(pc)
			}
		default:
		}
		return nil, ErrPoolTimeout

	case <-ctx.Done():
		p.mu.Lock()
		p.removeWaiter(ch)
		p.mu.Unlock()
		select {
		case pc := <-ch:
			if pc != nil {
				p.put(pc)
			}
		default:
		}
		return nil, ctx.Err()
	}
}

// put returns a connection to the pool for reuse.
func (p *smtpPool) put(pc *pooledConn) {
	p.mu.Lock()

	if p.closed {
		p.numOpen--
		p.mu.Unlock()
		p.closeConn(pc)
		return
	}

	pc.lastUsed = time.Now()

	// Discard expired or over-limit connections
	if p.isExpired(pc) {
		p.numOpen--
		p.wakeWaiter()
		p.mu.Unlock()
		p.closeConn(pc)
		return
	}

	// Hand off to a waiting caller if any
	if len(p.waitQueue) > 0 {
		ch := p.waitQueue[0]
		p.waitQueue = p.waitQueue[1:]
		p.mu.Unlock()
		ch <- pc
		return
	}

	// Store in idle pool if room
	if len(p.idle) < p.maxIdleCount {
		p.idle = append(p.idle, pc)
		p.mu.Unlock()
		return
	}

	// Idle pool full — discard
	p.numOpen--
	p.mu.Unlock()
	p.closeConn(pc)
}

// discard removes a connection from the pool without returning it.
// Call this when a send fails and the connection is unusable.
func (p *smtpPool) discard(pc *pooledConn) {
	p.mu.Lock()
	p.numOpen--
	p.wakeWaiter()
	p.mu.Unlock()
	p.closeConn(pc)
}

// healthCheck verifies that a pooled connection is still usable by sending RSET.
func (p *smtpPool) healthCheck(pc *pooledConn) error {
	if err := pc.conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if err := pc.client.Reset(); err != nil {
		return err
	}
	return pc.conn.SetDeadline(time.Time{}) // clear deadline
}

// closeConn gracefully closes a pooled connection.
func (p *smtpPool) closeConn(pc *pooledConn) {
	_ = pc.conn.SetDeadline(time.Now().Add(3 * time.Second))
	_ = pc.client.Quit()
	_ = pc.conn.Close()
}

// isExpired checks if a connection has exceeded its lifetime or message limit.
func (p *smtpPool) isExpired(pc *pooledConn) bool {
	if p.maxLife > 0 && time.Since(pc.createdAt) > p.maxLife {
		return true
	}
	if p.maxMsgs > 0 && pc.msgCount >= p.maxMsgs {
		return true
	}
	return false
}

// wakeWaiter signals the first waiter in the queue to dial a new connection.
// Must be called with mu held.
func (p *smtpPool) wakeWaiter() {
	if len(p.waitQueue) > 0 {
		// Instead of handing a nil, we increment numOpen for the waiter
		// and let them try to get a connection on retry.
		// But for simplicity, we just close the channel entry so the waiter retries.
		ch := p.waitQueue[0]
		p.waitQueue = p.waitQueue[1:]
		// Bump numOpen so the waiter can dial
		p.numOpen++
		close(ch)
	}
}

// removeWaiter removes a specific waiter channel from the queue.
// Must be called with mu held.
func (p *smtpPool) removeWaiter(ch chan *pooledConn) {
	for i, w := range p.waitQueue {
		if w == ch {
			p.waitQueue = append(p.waitQueue[:i], p.waitQueue[i+1:]...)
			return
		}
	}
}

// cleaner runs periodically to evict idle connections.
func (p *smtpPool) cleaner() {
	interval := p.maxIdleTime / 2
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanIdleConns()
		case <-p.cleanerCh:
			return
		}
	}
}

// cleanIdleConns removes expired idle connections.
func (p *smtpPool) cleanIdleConns() {
	now := time.Now()

	p.mu.Lock()
	var toClose []*pooledConn
	alive := p.idle[:0] // reuse underlying array
	for _, pc := range p.idle {
		if now.Sub(pc.lastUsed) > p.maxIdleTime || now.Sub(pc.createdAt) > p.maxLife {
			toClose = append(toClose, pc)
			p.numOpen--
		} else {
			alive = append(alive, pc)
		}
	}
	p.idle = alive
	p.mu.Unlock()

	for _, pc := range toClose {
		p.logger.Debug("pool cleaner evicting idle connection",
			"idle_time", now.Sub(pc.lastUsed).String(),
			"lifetime", now.Sub(pc.createdAt).String(),
		)
		p.closeConn(pc)
	}
}

// close shuts down the pool. Subsequent get calls return ErrPoolClosed.
// Active (checked-out) connections are cleaned up when put() sees closed.
func (p *smtpPool) close() error {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil
	}

	p.closed = true
	close(p.cleanerCh)

	// Collect idle connections to close
	toClose := p.idle
	p.idle = nil

	// Close all waiter channels (signals ErrPoolClosed via nil receive)
	for _, ch := range p.waitQueue {
		close(ch)
	}
	p.waitQueue = nil

	p.numOpen -= len(toClose)
	p.mu.Unlock()

	// Close connections outside the lock
	for _, pc := range toClose {
		p.closeConn(pc)
	}

	return nil
}
