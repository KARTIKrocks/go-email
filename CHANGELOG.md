# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-02-21

### Added
- **SMTP Connection Pooling** — opt-in connection reuse for high-throughput sending
  - New `PoolSize` config field enables pooling (0 = disabled, preserving backward compatibility)
  - LIFO idle stack with background cleaner for idle eviction
  - Wait queue with configurable timeout when pool is exhausted
  - Health checks (RSET) on checkout to detect stale connections
  - Automatic connection rotation via `MaxMessages` and `PoolMaxLifetime`
  - Configurable: `MaxIdleConns`, `PoolMaxLifetime`, `PoolMaxIdleTime`, `MaxMessages`, `PoolWaitTimeout`
  - New sentinel errors: `ErrPoolClosed`, `ErrPoolTimeout`
- Pool config validation in `SMTPConfig.Validate()`

## [1.0.0] - 2026-02-10

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
- HTML sanitization for user-provided content
- More template loading options (from directories, embed.FS)
- Webhook support for delivery notifications
- Internationalization support (i18n)
