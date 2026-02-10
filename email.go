package email

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

var (
	// ErrNoRecipients is returned when no recipients are specified
	ErrNoRecipients = errors.New("email: no recipients specified")

	// ErrNoSender is returned when no sender is specified
	ErrNoSender = errors.New("email: no sender specified")

	// ErrNoSubject is returned when no subject is specified
	ErrNoSubject = errors.New("email: no subject specified")

	// ErrNoBody is returned when no body is specified
	ErrNoBody = errors.New("email: no body specified")

	// ErrInvalidHeader is returned when a header contains invalid characters
	ErrInvalidHeader = errors.New("email: invalid header")
)

// Sender defines the interface for email senders
type Sender interface {
	// Send sends an email
	Send(ctx context.Context, email *Email) error

	// Close closes the sender connection
	Close() error
}

// Email represents an email message
type Email struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	ReplyTo     string
	Subject     string
	Body        string
	HTMLBody    string
	Attachments []Attachment
	Headers     map[string]string
	err         error // accumulated errors during building
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// NewEmail creates a new email
func NewEmail() *Email {
	return &Email{
		To:          []string{},
		Cc:          []string{},
		Bcc:         []string{},
		Attachments: []Attachment{},
		Headers:     make(map[string]string),
	}
}

// SetFrom sets the sender
func (e *Email) SetFrom(from string) *Email {
	if e.err != nil {
		return e
	}
	e.From = from
	return e
}

// AddTo adds recipients
func (e *Email) AddTo(to ...string) *Email {
	if e.err != nil {
		return e
	}
	e.To = append(e.To, to...)
	return e
}

// AddCc adds CC recipients
func (e *Email) AddCc(cc ...string) *Email {
	if e.err != nil {
		return e
	}
	e.Cc = append(e.Cc, cc...)
	return e
}

// AddBcc adds BCC recipients
func (e *Email) AddBcc(bcc ...string) *Email {
	if e.err != nil {
		return e
	}
	e.Bcc = append(e.Bcc, bcc...)
	return e
}

// SetReplyTo sets the reply-to address
func (e *Email) SetReplyTo(replyTo string) *Email {
	if e.err != nil {
		return e
	}
	e.ReplyTo = replyTo
	return e
}

// SetSubject sets the subject
func (e *Email) SetSubject(subject string) *Email {
	if e.err != nil {
		return e
	}
	e.Subject = subject
	return e
}

// SetBody sets the plain text body
func (e *Email) SetBody(body string) *Email {
	if e.err != nil {
		return e
	}
	e.Body = body
	return e
}

// SetHTMLBody sets the HTML body
func (e *Email) SetHTMLBody(html string) *Email {
	if e.err != nil {
		return e
	}
	e.HTMLBody = html
	return e
}

// AddAttachment adds an attachment
func (e *Email) AddAttachment(filename, contentType string, data []byte) *Email {
	if e.err != nil {
		return e
	}
	e.Attachments = append(e.Attachments, Attachment{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	})
	return e
}

// AddHeader adds a custom header
func (e *Email) AddHeader(key, value string) *Email {
	if e.err != nil {
		return e
	}
	if err := validateHeaderField(key, value); err != nil {
		e.err = err
		return e
	}
	e.Headers[key] = value
	return e
}

// Build validates the email and returns it or an error
func (e *Email) Build() (*Email, error) {
	if e.err != nil {
		return nil, e.err
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e, nil
}

// Validate validates the email
func (e *Email) Validate() error {
	// Validate sender
	if e.From == "" {
		return ErrNoSender
	}
	if _, err := mail.ParseAddress(e.From); err != nil {
		return fmt.Errorf("invalid from address %q: %w", e.From, err)
	}

	// Check recipients
	if len(e.To) == 0 && len(e.Cc) == 0 && len(e.Bcc) == 0 {
		return ErrNoRecipients
	}

	// Validate all recipients
	for _, addr := range e.To {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("invalid to address %q: %w", addr, err)
		}
	}
	for _, addr := range e.Cc {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("invalid cc address %q: %w", addr, err)
		}
	}
	for _, addr := range e.Bcc {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("invalid bcc address %q: %w", addr, err)
		}
	}

	// Validate reply-to if set
	if e.ReplyTo != "" {
		if _, err := mail.ParseAddress(e.ReplyTo); err != nil {
			return fmt.Errorf("invalid reply-to address %q: %w", e.ReplyTo, err)
		}
	}

	// Validate subject for header injection
	if err := validateHeaderField("Subject", e.Subject); err != nil {
		return err
	}
	if e.Subject == "" {
		return ErrNoSubject
	}

	// Validate body
	if e.Body == "" && e.HTMLBody == "" {
		return ErrNoBody
	}

	return nil
}

// validateHeaderField validates a header key-value pair for security
func validateHeaderField(key, value string) error {
	// Check for CRLF injection in key
	if strings.ContainsAny(key, "\r\n:") {
		return fmt.Errorf("%w: key contains invalid characters", ErrInvalidHeader)
	}
	// Check for CRLF injection in value
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%w: value contains invalid characters", ErrInvalidHeader)
	}
	return nil
}

// Error represents an email operation error with context
type Error struct {
	Op   string   // operation that failed (e.g., "send", "validate")
	From string   // sender address
	To   []string // recipient addresses
	Err  error    // underlying error
}

func (e *Error) Error() string {
	if len(e.To) > 0 {
		return fmt.Sprintf("email %s: from=%s to=%v: %v", e.Op, e.From, e.To, e.Err)
	}
	return fmt.Sprintf("email %s: from=%s: %v", e.Op, e.From, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}
