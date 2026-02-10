# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-XX

### Added
- Initial release
- SMTP email sending with TLS/STARTTLS support
- HTML and plain text email bodies
- Email templates using Go's `text/template` and `html/template`
- File attachment support with proper MIME encoding
- Automatic retry with exponential backoff
- Built-in rate limiting
- Pluggable logger interface with slog adapter
- Context support for timeouts and cancellation
- Email address validation using `net/mail`
- Email header injection protection
- Mock sender for testing
- Batch sending with concurrency control
- Fluent builder API for constructing emails
- Multiple recipient support (To, Cc, Bcc)
- Custom email headers
- Reply-To support
- Minimal external dependencies (only uses Go standard library + x/time and x/sync)

### Security
- Email header injection protection (validates all headers for CRLF)
- Email address validation to prevent malformed addresses
- TLS/STARTTLS support for encrypted connections

## [Unreleased]

### Planned
- DKIM signing support
- Connection pooling for high-volume sending
- HTML sanitization for user-provided content
- More template loading options (from directories, embed.FS)
- Webhook support for delivery notifications
- Internationalization support (i18n)
