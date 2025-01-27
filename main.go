package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
)

var (
	secretKey       string
	hashedSecretKey []byte
)

type Config struct {
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUsername string `json:"smtpUsername"`
	SMTPPassword string `json:"smtpPassword"`

	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Fields  []string `json:"fields"`
}

func (c *Config) Check() error {
	if c.SMTPHost == "" {
		return fmt.Errorf("%q parameter is required", "smtpHost")
	}
	if c.SMTPPort <= 0 {
		return fmt.Errorf("%q parameter is required", "smtpPort")
	}
	if c.SMTPUsername == "" {
		return fmt.Errorf("%q parameter is required", "smtpUsername")
	}
	if c.SMTPPassword == "" {
		return fmt.Errorf("%q parameter is required", "smtpPassword")
	}
	if c.From == "" {
		return fmt.Errorf("%q parameter is required", "from")
	}
	if len(c.To) == 0 {
		return fmt.Errorf("%q parameter is required", "to")
	}
	if c.Subject == "" {
		return fmt.Errorf("%q parameter is required", "subject")
	}
	if len(c.Fields) == 0 {
		return fmt.Errorf("%q parameter is required", "fields")
	}
	return nil
}

func main() {
	// Secret key init
	secretKey = os.Getenv("SECRET")
	if secretKey == "" {
		log.Fatal("SECRET env var not set")
	}
	hashedSecretKey = hashKey(secretKey)

	// Encrypt JSON config for client-side future usage
	// Usage:
	//   curl https://<HOST>/encrypt\?key\=<SECRET_KEY> -X POST -H "Content-Type: application/json" -d '{"smtpHost":"<SMTP_HOST>","smtpPort":<SMTP_PORT>,"smtpUsername":"<SMTP_USER>","smtpPassword":"<SMTP_PASSWORD>","from":"<FROM_EMAIL>","to":["<TO_EMAIL>"],"subject":"<SUBJECT>","fields":["<FIELD1>","<FIELD2>"]}'
	http.HandleFunc("/encrypt", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("key") != secretKey {
			http.Error(w, "Invalid secret key", http.StatusUnauthorized)
			return
		}

		// Decode JSON
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Failed to decode JSON config: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check config
		if err := config.Check(); err != nil {
			http.Error(w, "Invalid config: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Create a clean JSON string
		b, err := json.Marshal(&config)
		if err != nil {
			http.Error(w, "Error while cenverting config struct back to bytes: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Encrypt JSON
		encrypted, err := encrypt(string(b))
		if err != nil {
			http.Error(w, "Error while encrypting JSON config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Send base64 encrypted JSON
		fmt.Fprint(w, encrypted)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		encryptedConfig := r.FormValue("config")
		if encryptedConfig == "" {
			http.Error(w, "No config provided", http.StatusBadRequest)
			return
		}

		// Decrypt JSON
		decryptedConfig, err := decrypt(r.FormValue(encryptedConfig))
		if err != nil {
			http.Error(w, "Failed to decrypt data: "+err.Error(), http.StatusBadRequest)
			return
		}

		var config Config
		if err := json.NewDecoder(strings.NewReader(decryptedConfig)).Decode(&config); err != nil {
			http.Error(w, "Failed to decode JSON data: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check config
		if err := config.Check(); err != nil {
			http.Error(w, "Invalid config: "+err.Error(), http.StatusBadRequest)
			return
		}

		data := make(map[string]string)
		for _, name := range config.Fields {
			data[name] = r.FormValue(name)
		}

		if err := sendEmail(config, data); err != nil {
			log.Printf("Failed to send email: %v", err)
			http.Error(w, "Failed to send email", http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, "Form submitted successfully")
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

func sendEmail(config Config, data map[string]string) error {
	message := "From: " + config.From + "\r\n" +
		"To: " + strings.Join(config.To, ",") + "\r\n" +
		"Subject: " + config.Subject + "\r\n" +
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n"

	for key, value := range data {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	return smtp.SendMail(
		fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort),
		smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost),
		config.From,
		config.To,
		[]byte(message),
	)
}
