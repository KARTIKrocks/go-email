# Contributing to go-email

Thank you for considering contributing to go-email! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/KARTIKrocks/go-email/issues)
2. If not, create a new issue with:
   - Clear title and description
   - Steps to reproduce
   - Expected vs actual behavior
   - Go version and OS
   - Code sample if applicable

### Suggesting Features

1. Check [Issues](https://github.com/KARTIKrocks/go-email/issues) for existing feature requests
2. Create a new issue with:
   - Clear description of the feature
   - Use cases and benefits
   - Possible implementation approach (optional)

### Submitting Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes following our coding standards
4. Add/update tests for your changes
5. Ensure all tests pass: `go test ./...`
6. Update documentation if needed
7. Commit with clear messages: `git commit -m 'Add amazing feature'`
8. Push to your fork: `git push origin feature/amazing-feature`
9. Open a Pull Request

## Development Setup

```bash
# Clone your fork
git clone https://github.com/KARTIKrocks/go-email.git
cd go-email

# Install dependencies
go mod download

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...
```

## Coding Standards

### Code Style

- Follow standard Go formatting: `gofmt -s -w .`
- Use `go vet` to check for common mistakes
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Keep functions focused and small
- Use meaningful variable and function names

### Documentation

- Add godoc comments for all exported types, functions, and constants
- Include examples in documentation where helpful
- Update README.md for user-facing changes

### Testing

- Write tests for new functionality
- Maintain or improve code coverage
- Test edge cases and error conditions
- Use table-driven tests where appropriate

Example test structure:

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Commit Messages

Use clear, descriptive commit messages:

```
Add rate limiting to SMTP sender

- Implement token bucket rate limiter
- Add RateLimit config option
- Update tests and documentation
```

## Project Structure

```
go-email/
├── doc.go              # Package documentation
├── email.go            # Core email types
├── smtp.go             # SMTP sender implementation
├── mailer.go           # High-level mailer interface
├── template.go         # Template support
├── logger.go           # Logger interface
├── logger_slog.go      # Slog adapter
├── mock.go             # Mock sender for testing
├── email_test.go       # Tests
├── examples/           # Example code
│   ├── basic/
│   ├── template/
│   ├── attachment/
│   ├── batch/
│   └── testing/
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── go.mod
```

## Pull Request Checklist

Before submitting a PR, ensure:

- [ ] Code follows Go conventions and passes `go fmt`, `go vet`
- [ ] All tests pass: `go test ./...`
- [ ] New code has tests with good coverage
- [ ] Documentation is updated (godoc, README)
- [ ] Commit messages are clear and descriptive
- [ ] No breaking changes (or clearly documented)
- [ ] Examples updated if API changed

## Questions?

Feel free to open an issue for questions or discussions!

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
