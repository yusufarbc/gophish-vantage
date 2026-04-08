package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gophish/gophish/config"
	log "github.com/gophish/gophish/logger"
)

var conf *config.NotificationConfig

// Setup initializes the notifier package with the provided configuration.
func Setup(c *config.NotificationConfig) {
	conf = c
}

// SendAlert sends a notification alert for a finding.
func SendAlert(toolName, severity, name, target string) {
	if conf == nil || !conf.Enabled {
		return
	}

	// Only alert on high/critical findings
	sevUpper := strings.ToUpper(severity)
	if sevUpper != "CRITICAL" && sevUpper != "HIGH" {
		return
	}

	msg := fmt.Sprintf("🚨 *VANTAGE ALERT* 🚨\n\n*Severity:* %s\n*Tool:* %s\n*Finding:* %s\n*Target:* %s\n*Time:* %s",
		sevUpper, toolName, name, target, time.Now().Format("2006-01-02 15:04:05"))

	// 1. Try Notify binary if configured
	if conf.NotifyConfig != "" {
		go callNotify(msg)
	}

	// 2. Fallback/Direct pushing to Slack
	if conf.SlackWebhook != "" {
		go sendSlack(msg)
	}

	// 3. Direct pushing to Telegram
	if conf.TelegramToken != "" && conf.TelegramChatID != "" {
		go sendTelegram(msg)
	}
}

func callNotify(msg string) {
	// Example: echo "msg" | notify -config config/notify.yaml
	cmd := exec.Command("notify", "-config", conf.NotifyConfig, "-silent")
	cmd.Stdin = bytes.NewBufferString(msg)
	if err := cmd.Run(); err != nil {
		log.Errorf("notify tool error: %v", err)
	}
}

func sendSlack(msg string) {
	payload := map[string]string{"text": msg}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(conf.SlackWebhook, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("slack notification error: %v", err)
		return
	}
	defer resp.Body.Close()
}

func sendTelegram(msg string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", conf.TelegramToken)
	payload := map[string]string{
		"chat_id":    conf.TelegramChatID,
		"text":       msg,
		"parse_mode": "Markdown",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("telegram notification error: %v", err)
		return
	}
	defer resp.Body.Close()
}
