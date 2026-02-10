package email

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Mailer provides a high-level email sending interface.
// It is safe for concurrent use.
type Mailer struct {
	sender    Sender
	from      string
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewMailer creates a new mailer
func NewMailer(sender Sender, from string) *Mailer {
	return &Mailer{
		sender:    sender,
		from:      from,
		templates: make(map[string]*Template),
	}
}

// RegisterTemplate registers an email template.
// It is safe for concurrent use.
func (m *Mailer) RegisterTemplate(name string, tmpl *Template) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[name] = tmpl
}

// Send sends a simple email
func (m *Mailer) Send(ctx context.Context, to []string, subject, body string) error {
	email := NewEmail().
		SetFrom(m.from).
		AddTo(to...).
		SetSubject(subject).
		SetBody(body)

	builtEmail, err := email.Build()
	if err != nil {
		return err
	}

	return m.sender.Send(ctx, builtEmail)
}

// SendHTML sends an HTML email
func (m *Mailer) SendHTML(ctx context.Context, to []string, subject, html string) error {
	email := NewEmail().
		SetFrom(m.from).
		AddTo(to...).
		SetSubject(subject).
		SetHTMLBody(html)

	builtEmail, err := email.Build()
	if err != nil {
		return err
	}

	return m.sender.Send(ctx, builtEmail)
}

// SendTemplate sends an email using a registered template
func (m *Mailer) SendTemplate(ctx context.Context, to []string, templateName string, data any) error {
	m.mu.RLock()
	tmpl, exists := m.templates[templateName]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("template not found: %s", templateName)
	}

	email, err := tmpl.Render(data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	email.SetFrom(m.from).AddTo(to...)

	builtEmail, err := email.Build()
	if err != nil {
		return err
	}

	return m.sender.Send(ctx, builtEmail)
}

// SendEmail sends a custom email
func (m *Mailer) SendEmail(ctx context.Context, email *Email) error {
	if email.From == "" {
		email.From = m.from
	}

	builtEmail, err := email.Build()
	if err != nil {
		return err
	}

	return m.sender.Send(ctx, builtEmail)
}

// SendBatch sends multiple emails concurrently with a concurrency limit.
// The concurrencyLimit parameter controls how many emails are sent simultaneously.
// If concurrencyLimit is <= 0, a default of 10 is used.
//
// All emails are validated before sending begins. If any email fails validation,
// the entire batch fails without sending any emails.
//
// If any email fails to send, the error is returned, but other emails may still
// be sent concurrently. Use the returned error to check for failures.
func (m *Mailer) SendBatch(ctx context.Context, emails []*Email, concurrencyLimit int) error {
	if concurrencyLimit <= 0 {
		concurrencyLimit = 10
	}

	// Validate all emails first
	for i, email := range emails {
		if email.From == "" {
			email.From = m.from
		}
		if _, err := email.Build(); err != nil {
			return fmt.Errorf("email %d validation failed: %w", i, err)
		}
	}

	// Send emails concurrently
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrencyLimit)

	for _, email := range emails {
		g.Go(func() error {
			return m.sender.Send(ctx, email)
		})
	}

	return g.Wait()
}

// Close closes the mailer
func (m *Mailer) Close() error {
	return m.sender.Close()
}
