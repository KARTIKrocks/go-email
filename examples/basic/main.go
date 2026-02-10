package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/KARTIKrocks/go-email"
)

func main() {
	// Configure SMTP
	config := email.SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),     // e.g., "smtp.gmail.com"
		Port:     587,                        // Use 587 for TLS
		Username: os.Getenv("SMTP_USERNAME"), // Your email
		Password: os.Getenv("SMTP_PASSWORD"), // App password for Gmail
		From:     os.Getenv("SMTP_FROM"),     // Default sender
		UseTLS:   true,
	}

	// Create sender
	sender, err := email.NewSMTPSender(config)
	if err != nil {
		log.Fatalf("Failed to create sender: %v", err)
	}
	mailer := email.NewMailer(sender, config.From)
	defer mailer.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send simple email
	err = mailer.Send(ctx,
		[]string{"recipient@example.com"},
		"Hello from Go!",
		"This is a test email sent from the go-email package.",
	)

	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}

	log.Println("Email sent successfully!")
}
