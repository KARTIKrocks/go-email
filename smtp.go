package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// DefaultTimeout is the default timeout for SMTP operations
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries is the default number of retry attempts
	DefaultMaxRetries = 3

	// DefaultRetryDelay is the default initial retry delay
	DefaultRetryDelay = time.Second

	// DefaultRetryBackoff is the default exponential backoff multiplier
	DefaultRetryBackoff = 2.0

	// DefaultRateLimit is the default rate limit (emails per second)
	DefaultRateLimit = 10

	// boundaryPrefix is the prefix for MIME boundaries
	boundaryPrefix = "boundary-"

	// altBoundaryPrefix is the prefix for alternative content boundaries
	altBoundaryPrefix = "alt-boundary-"
)

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	// Host is the SMTP server hostname
	Host string

	// Port is the SMTP server port (typically 587 for TLS, 465 for SSL)
	Port int

	// Username is the SMTP authentication username
	Username string

	// Password is the SMTP authentication password
	Password string

	// From is the default sender email address
	From string

	// UseTLS enables STARTTLS encryption
	UseTLS bool

	// Timeout is the connection timeout (default: 30s)
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int

	// RetryDelay is the initial retry delay (default: 1s)
	RetryDelay time.Duration

	// RetryBackoff is the exponential backoff multiplier (default: 2.0)
	RetryBackoff float64

	// RateLimit is the maximum number of emails per second (default: 10)
	// Set to 0 to disable rate limiting
	RateLimit int

	// Logger is the logger interface for observability
	// If nil, logging is disabled (NoOpLogger used)
	Logger Logger
}

// Validate validates the SMTP configuration
func (c SMTPConfig) Validate() error {
	if c.Host == "" {
		return errors.New("smtp: host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("smtp: invalid port %d (must be 1-65535)", c.Port)
	}
	if c.Username == "" && c.Password != "" {
		return errors.New("smtp: password set but username is empty")
	}
	if c.Password == "" && c.Username != "" {
		return errors.New("smtp: username set but password is empty")
	}
	return nil
}

// SMTPSender sends emails via SMTP
type SMTPSender struct {
	config  SMTPConfig
	logger  Logger
	limiter *rate.Limiter
}

// NewSMTPSender creates a new SMTP sender.
// It validates the config and returns an error if it is invalid.
func NewSMTPSender(config SMTPConfig) (*SMTPSender, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = DefaultMaxRetries
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = DefaultRetryDelay
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = DefaultRetryBackoff
	}
	if config.RateLimit == 0 {
		config.RateLimit = DefaultRateLimit
	}

	// Set logger
	logger := config.Logger
	if logger == nil {
		logger = NoOpLogger{}
	}

	// Create rate limiter
	var limiter *rate.Limiter
	if config.RateLimit > 0 {
		limiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(config.RateLimit)), config.RateLimit)
	}

	return &SMTPSender{
		config:  config,
		logger:  logger,
		limiter: limiter,
	}, nil
}

// Send sends an email via SMTP with retry logic
func (s *SMTPSender) Send(ctx context.Context, email *Email) error {
	// Set default from if not specified
	if email.From == "" {
		email.From = s.config.From
	}

	// Validate email
	if err := email.Validate(); err != nil {
		return &Error{
			Op:   "validate",
			From: email.From,
			To:   email.To,
			Err:  err,
		}
	}

	// Apply rate limiting
	if s.limiter != nil {
		if err := s.limiter.Wait(ctx); err != nil {
			s.logger.Error("rate limit error", "error", err)
			return &Error{
				Op:   "rate_limit",
				From: email.From,
				To:   email.To,
				Err:  err,
			}
		}
	}

	// Retry logic with exponential backoff
	var lastErr error
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return &Error{
				Op:   "send",
				From: email.From,
				To:   email.To,
				Err:  ctx.Err(),
			}
		default:
		}

		// Wait before retry (skip on first attempt)
		if attempt > 0 {
			delay := time.Duration(float64(s.config.RetryDelay) *
				math.Pow(s.config.RetryBackoff, float64(attempt-1)))

			s.logger.Warn("retrying email send",
				"attempt", attempt,
				"max_retries", s.config.MaxRetries,
				"delay", delay.String(),
				"to", email.To,
			)

			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return &Error{
					Op:   "send",
					From: email.From,
					To:   email.To,
					Err:  ctx.Err(),
				}
			}
		}

		// Attempt to send
		lastErr = s.sendOnce(ctx, email)
		if lastErr == nil {
			s.logger.Info("email sent successfully",
				"to", email.To,
				"subject", email.Subject,
				"attempt", attempt+1,
			)
			return nil
		}

		// Don't retry validation errors
		var emailErr *Error
		if errors.As(lastErr, &emailErr) {
			if errors.Is(emailErr.Err, ErrNoRecipients) ||
				errors.Is(emailErr.Err, ErrNoSender) ||
				errors.Is(emailErr.Err, ErrNoSubject) ||
				errors.Is(emailErr.Err, ErrNoBody) {
				return lastErr
			}
		}

		s.logger.Warn("email send attempt failed",
			"attempt", attempt+1,
			"error", lastErr,
			"to", email.To,
		)
	}

	s.logger.Error("email send failed after all retries",
		"attempts", s.config.MaxRetries+1,
		"error", lastErr,
		"to", email.To,
	)

	return &Error{
		Op:   "send",
		From: email.From,
		To:   email.To,
		Err:  fmt.Errorf("failed after %d attempts: %w", s.config.MaxRetries+1, lastErr),
	}
}

// sendOnce attempts to send an email once (no retries)
func (s *SMTPSender) sendOnce(ctx context.Context, email *Email) error {
	// Build message
	message, err := s.buildMessage(email)
	if err != nil {
		return &Error{
			Op:   "build_message",
			From: email.From,
			To:   email.To,
			Err:  err,
		}
	}

	// Get recipients
	recipients := append([]string{}, email.To...)
	recipients = append(recipients, email.Cc...)
	recipients = append(recipients, email.Bcc...)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.logger.Debug("connecting to SMTP server",
		"host", s.config.Host,
		"port", s.config.Port,
		"tls", s.config.UseTLS,
	)

	// Setup authentication
	var auth smtp.Auth
	if s.config.Username != "" && s.config.Password != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	// Send email
	if s.config.UseTLS {
		return s.sendWithTLS(ctx, addr, auth, email.From, recipients, message)
	}

	return s.sendPlain(ctx, addr, auth, email.From, recipients, message)
}

// sendPlain sends email without TLS using a context-aware dialer
func (s *SMTPSender) sendPlain(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	dialer := &net.Dialer{
		Timeout: s.config.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("auth: %w", authErr)
		}
	}

	if mailErr := client.Mail(from); mailErr != nil {
		return fmt.Errorf("set sender: %w", mailErr)
	}

	for _, recipient := range to {
		if rcptErr := client.Rcpt(recipient); rcptErr != nil {
			return fmt.Errorf("add recipient %s: %w", recipient, rcptErr)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("start data: %w", err)
	}

	if _, err = w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("write data: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return client.Quit()
}

// sendWithTLS sends email with STARTTLS
func (s *SMTPSender) sendWithTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	dialer := &net.Dialer{
		Timeout: s.config.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
	}

	if tlsErr := client.StartTLS(tlsConfig); tlsErr != nil {
		return fmt.Errorf("start tls: %w", tlsErr)
	}

	s.logger.Debug("TLS connection established")

	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("auth: %w", authErr)
		}
		s.logger.Debug("authentication successful")
	}

	if mailErr := client.Mail(from); mailErr != nil {
		return fmt.Errorf("set sender: %w", mailErr)
	}

	for _, recipient := range to {
		if rcptErr := client.Rcpt(recipient); rcptErr != nil {
			return fmt.Errorf("add recipient %s: %w", recipient, rcptErr)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("start data: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("write data: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return client.Quit()
}

// buildMessage builds the email message with proper MIME encoding
func (s *SMTPSender) buildMessage(email *Email) ([]byte, error) {
	buf := &strings.Builder{}

	// Headers
	fmt.Fprintf(buf, "From: %s\r\n", email.From)
	fmt.Fprintf(buf, "To: %s\r\n", strings.Join(email.To, ", "))

	if len(email.Cc) > 0 {
		fmt.Fprintf(buf, "Cc: %s\r\n", strings.Join(email.Cc, ", "))
	}

	if email.ReplyTo != "" {
		fmt.Fprintf(buf, "Reply-To: %s\r\n", email.ReplyTo)
	}

	fmt.Fprintf(buf, "Subject: %s\r\n", encodeHeader(email.Subject))

	// Add Message-ID if not present
	if _, exists := email.Headers["Message-ID"]; !exists {
		msgID := fmt.Sprintf("<%s@%s>", generateUniqueID(), s.config.Host)
		fmt.Fprintf(buf, "Message-ID: %s\r\n", msgID)
	}

	// Add Date header
	fmt.Fprintf(buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))

	// Custom headers
	for key, value := range email.Headers {
		fmt.Fprintf(buf, "%s: %s\r\n", key, value)
	}

	// MIME headers
	buf.WriteString("MIME-Version: 1.0\r\n")

	// Check if we need multipart
	hasAttachments := len(email.Attachments) > 0
	hasHTML := email.HTMLBody != ""

	switch {
	case hasAttachments || (email.Body != "" && hasHTML):
		boundary := boundaryPrefix + generateUniqueID()
		fmt.Fprintf(buf, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
		buf.WriteString("\r\n")

		// Body parts
		switch {
		case email.Body != "" && hasHTML:
			// Multipart alternative
			altBoundary := altBoundaryPrefix + generateUniqueID()
			fmt.Fprintf(buf, "--%s\r\n", boundary)
			fmt.Fprintf(buf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary)

			// Plain text
			fmt.Fprintf(buf, "--%s\r\n", altBoundary)
			buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
			buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			buf.WriteString(quotedPrintableEncode(email.Body))
			buf.WriteString("\r\n\r\n")

			// HTML
			fmt.Fprintf(buf, "--%s\r\n", altBoundary)
			buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
			buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			buf.WriteString(quotedPrintableEncode(email.HTMLBody))
			buf.WriteString("\r\n\r\n")
			fmt.Fprintf(buf, "--%s--\r\n", altBoundary)
		case hasHTML:
			fmt.Fprintf(buf, "--%s\r\n", boundary)
			buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
			buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			buf.WriteString(quotedPrintableEncode(email.HTMLBody))
			buf.WriteString("\r\n\r\n")
		default:
			fmt.Fprintf(buf, "--%s\r\n", boundary)
			buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
			buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			buf.WriteString(quotedPrintableEncode(email.Body))
			buf.WriteString("\r\n\r\n")
		}

		// Attachments
		for _, att := range email.Attachments {
			fmt.Fprintf(buf, "--%s\r\n", boundary)
			fmt.Fprintf(buf, "Content-Type: %s\r\n", att.ContentType)
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(buf, "Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", sanitizeFilename(att.Filename))

			// Encode and wrap base64 at 76 characters per RFC 2045
			encoded := base64.StdEncoding.EncodeToString(att.Data)
			buf.WriteString(wrapText(encoded, 76))
			buf.WriteString("\r\n\r\n")
		}

		fmt.Fprintf(buf, "--%s--\r\n", boundary)
	case hasHTML:
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		buf.WriteString(quotedPrintableEncode(email.HTMLBody))
	default:
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		buf.WriteString(quotedPrintableEncode(email.Body))
	}

	return []byte(buf.String()), nil
}

// Close closes the SMTP sender
func (s *SMTPSender) Close() error {
	return nil
}

// wrapText wraps text at the specified width
func wrapText(text string, width int) string {
	var result strings.Builder
	for i := 0; i < len(text); i += width {
		end := i + width
		if end > len(text) {
			end = len(text)
		}
		result.WriteString(text[i:end])
		if end < len(text) {
			result.WriteString("\r\n")
		}
	}
	return result.String()
}

// quotedPrintableEncode encodes text using quoted-printable encoding for safe email transport.
func quotedPrintableEncode(s string) string {
	var buf strings.Builder
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return buf.String()
}

// encodeHeader encodes a header value using RFC 2047 if it contains non-ASCII characters.
func encodeHeader(value string) string {
	for _, r := range value {
		if r > 127 {
			return mime.QEncoding.Encode("UTF-8", value)
		}
	}
	return value
}

// sanitizeFilename removes characters that could cause header injection in
// Content-Disposition filenames.
func sanitizeFilename(name string) string {
	// Remove characters that could break MIME headers
	replacer := strings.NewReplacer(
		"\"", "_",
		"\r", "",
		"\n", "",
		"\x00", "",
		"/", "_",
		"\\", "_",
	)
	return replacer.Replace(name)
}

// generateUniqueID generates a unique identifier for Message-ID and boundaries
func generateUniqueID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: should never happen with crypto/rand
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
