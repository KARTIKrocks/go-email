# Logger Adapters

The email package provides a simple `Logger` interface that allows you to integrate with any logging library. Below are examples of adapters for popular Go logging libraries.

## Using slog (Built-in)

The package includes a built-in adapter for Go's standard library `slog`:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

config := email.SMTPConfig{
    Host:   "smtp.gmail.com",
    Logger: email.NewSlogLogger(logger),
}
```

## Using Zap

```go
package main

import (
    "go.uber.org/zap"
    "github.com/KARTIKrocks/go-email"
)

type ZapLogger struct {
    logger *zap.SugaredLogger
}

func NewZapLogger(logger *zap.SugaredLogger) email.Logger {
    return &ZapLogger{logger: logger}
}

func (l *ZapLogger) Debug(msg string, keysAndValues ...any) {
    l.logger.Debugw(msg, keysAndValues...)
}

func (l *ZapLogger) Info(msg string, keysAndValues ...any) {
    l.logger.Infow(msg, keysAndValues...)
}

func (l *ZapLogger) Warn(msg string, keysAndValues ...any) {
    l.logger.Warnw(msg, keysAndValues...)
}

func (l *ZapLogger) Error(msg string, keysAndValues ...any) {
    l.logger.Errorw(msg, keysAndValues...)
}

func (l *ZapLogger) With(keysAndValues ...any) email.Logger {
    return &ZapLogger{logger: l.logger.With(keysAndValues...)}
}

// Usage
func main() {
    zapLogger, _ := zap.NewProduction()
    defer zapLogger.Sync()

    config := email.SMTPConfig{
        Host:   "smtp.gmail.com",
        Logger: NewZapLogger(zapLogger.Sugar()),
    }
}
```

## Using Logrus

```go
package main

import (
    "github.com/sirupsen/logrus"
    "github.com/KARTIKrocks/go-email"
)

type LogrusLogger struct {
    logger *logrus.Logger
}

func NewLogrusLogger(logger *logrus.Logger) email.Logger {
    return &LogrusLogger{logger: logger}
}

func (l *LogrusLogger) Debug(msg string, keysAndValues ...any) {
    l.logger.WithFields(kvToFields(keysAndValues...)).Debug(msg)
}

func (l *LogrusLogger) Info(msg string, keysAndValues ...any) {
    l.logger.WithFields(kvToFields(keysAndValues...)).Info(msg)
}

func (l *LogrusLogger) Warn(msg string, keysAndValues ...any) {
    l.logger.WithFields(kvToFields(keysAndValues...)).Warn(msg)
}

func (l *LogrusLogger) Error(msg string, keysAndValues ...any) {
    l.logger.WithFields(kvToFields(keysAndValues...)).Error(msg)
}

func (l *LogrusLogger) With(keysAndValues ...any) email.Logger {
    entry := l.logger.WithFields(kvToFields(keysAndValues...))
    return &LogrusLogger{logger: entry.Logger}
}

func kvToFields(keysAndValues ...any) logrus.Fields {
    fields := make(logrus.Fields)
    for i := 0; i < len(keysAndValues); i += 2 {
        if i+1 < len(keysAndValues) {
            key, ok := keysAndValues[i].(string)
            if ok {
                fields[key] = keysAndValues[i+1]
            }
        }
    }
    return fields
}

// Usage
func main() {
    logrusLogger := logrus.New()
    logrusLogger.SetLevel(logrus.DebugLevel)

    config := email.SMTPConfig{
        Host:   "smtp.gmail.com",
        Logger: NewLogrusLogger(logrusLogger),
    }
}
```

## Using Zerolog

```go
package main

import (
    "github.com/rs/zerolog"
    "github.com/KARTIKrocks/go-email"
)

type ZerologLogger struct {
    logger zerolog.Logger
}

func NewZerologLogger(logger zerolog.Logger) email.Logger {
    return &ZerologLogger{logger: logger}
}

func (l *ZerologLogger) Debug(msg string, keysAndValues ...any) {
    l.logWithLevel(l.logger.Debug(), msg, keysAndValues...)
}

func (l *ZerologLogger) Info(msg string, keysAndValues ...any) {
    l.logWithLevel(l.logger.Info(), msg, keysAndValues...)
}

func (l *ZerologLogger) Warn(msg string, keysAndValues ...any) {
    l.logWithLevel(l.logger.Warn(), msg, keysAndValues...)
}

func (l *ZerologLogger) Error(msg string, keysAndValues ...any) {
    l.logWithLevel(l.logger.Error(), msg, keysAndValues...)
}

func (l *ZerologLogger) With(keysAndValues ...any) email.Logger {
    ctx := l.logger.With()
    for i := 0; i < len(keysAndValues); i += 2 {
        if i+1 < len(keysAndValues) {
            if key, ok := keysAndValues[i].(string); ok {
                ctx = ctx.Interface(key, keysAndValues[i+1])
            }
        }
    }
    return &ZerologLogger{logger: ctx.Logger()}
}

func (l *ZerologLogger) logWithLevel(event *zerolog.Event, msg string, keysAndValues ...any) {
    for i := 0; i < len(keysAndValues); i += 2 {
        if i+1 < len(keysAndValues) {
            if key, ok := keysAndValues[i].(string); ok {
                event = event.Interface(key, keysAndValues[i+1])
            }
        }
    }
    event.Msg(msg)
}

// Usage
func main() {
    zerologLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

    config := email.SMTPConfig{
        Host:   "smtp.gmail.com",
        Logger: NewZerologLogger(zerologLogger),
    }
}
```

## Creating Your Own Adapter

To create an adapter for your preferred logging library:

1. Implement the `email.Logger` interface:

```go
type Logger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
    With(keysAndValues ...any) Logger
}
```

2. Convert the key-value pairs to your logger's format

3. Use it in the config:

```go
config := email.SMTPConfig{
    Logger: YourCustomLogger{},
}
```

## Disabling Logging

To disable logging entirely (default behavior):

```go
config := email.SMTPConfig{
    Logger: nil, // or simply omit the Logger field
}
```

Or explicitly use the NoOpLogger:

```go
config := email.SMTPConfig{
    Logger: email.NoOpLogger{},
}
```
