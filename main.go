// main.go
// main.go (New version using the public API)
package main

import (
	"encoding/json"
"fmt"
	"io"
"log"
	"net/http"
"os"
	"os/exec"
"strconv"
"strings"
"sync"
	"time"

tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
"github.com/robfig/cron/v3"
)

const domainsFilePath = "/data/domains.txt" 
const domainsFilePath = "/data/domains.txt"
const apiBaseURL = "https://check.skiddle.id/"
const maxDomainsPerRequest = 30 // As per the API documentation

var fileMutex = &sync.Mutex{}

// This struct helps us parse the JSON response from the API
type DomainStatus struct {
	Blocked bool `json:"blocked"`
}

func main() {
botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Panic("TELEGRAM_BOT_TOKEN not set")
	}
adminChatIDStr := os.Getenv("ADMIN_CHAT_ID")
	if adminChatIDStr == "" {
		log.Panic("ADMIN_CHAT_ID not set")
	}
	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		log.Panic("Invalid ADMIN_CHAT_ID")
	}
	adminChatID, _ := strconv.ParseInt(adminChatIDStr, 10, 64)

bot, err := tgbotapi.NewBotAPI(botToken)
if err != nil {
@@ -38,32 +40,28 @@ func main() {
bot.Debug = true
log.Printf("Authorized on account %s", bot.Self.UserName)

	// Ensure the data directory and domains.txt file exist
if err := os.MkdirAll("/data", 0755); err != nil {
log.Panic("Failed to create data directory: ", err)
}
if _, err := os.Stat(domainsFilePath); os.IsNotExist(err) {
os.Create(domainsFilePath)
}
	

c := cron.New()
c.AddFunc("*/30 * * * *", func() {
log.Println("Running scheduled domain check...")
checkDomainsAndNotify(bot, adminChatID)
})
c.Start()
	log.Println("Cron scheduler started. Checks will run every 30 minutes.")
	
	sendMessage(bot, adminChatID, "✅ Bot started successfully! Scheduled checks are active.")

	sendMessage(bot, adminChatID, "✅ Bot started successfully! Using public API. Scheduled checks are active.")

u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
updates := bot.GetUpdatesChan(u)

for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != adminChatID {
		if update.Message == nil || update.Message.Chat.ID != adminChatID {
continue
}

@@ -78,25 +76,79 @@ func main() {
case "list":
msg.Text = listDomains()
case "checknow":
			msg.Text = "🚀 Starting manual check..."
			sendMessage(bot, adminChatID, msg.Text)
			sendMessage(bot, adminChatID, "🚀 Starting manual check via API...")
go checkDomainsAndNotify(bot, adminChatID)
continue
default:
msg.Text = "I don't know that command."
}
		sendMessage(bot, adminChatID, msg.Text)
	}
}

// The new check function that calls the web API
func checkDomainsAndNotify(bot *tgbotapi.BotAPI, chatID int64) {
	domains, err := readDomains()
	if err != nil {
		sendMessage(bot, chatID, fmt.Sprintf("🚨 Error reading domain list: %v", err))
		return
	}
	if len(domains) == 0 {
		sendMessage(bot, chatID, "📄 Domain list is empty. Nothing to check.")
		return
	}

	var resultsText strings.Builder
	resultsText.WriteString("📄 Domain Check Results:\n\n")

	// The API is limited to 30 domains per request, so we process in batches
	for i := 0; i < len(domains); i += maxDomainsPerRequest {
		end := i + maxDomainsPerRequest
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		url := fmt.Sprintf("%s?domains=%s&json=true", apiBaseURL, strings.Join(batch, ","))

		httpClient := &http.Client{Timeout: 15 * time.Second}
		resp, err := httpClient.Get(url)
		if err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to check batch: %v\n", err))
			continue
		}
		defer resp.Body.Close()

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Error sending message: %v", err)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to read API response: %v\n", err))
			continue
		}

		var statuses map[string]DomainStatus
		if err := json.Unmarshal(body, &statuses); err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to parse API JSON: %v\n", err))
			continue
		}

		for domain, status := range statuses {
			statusText := "✅ Not Blocked"
			if status.Blocked {
				statusText = "🚫 BLOCKED"
			}
			resultsText.WriteString(fmt.Sprintf("%s: %s\n", domain, statusText))
}
}

	sendMessage(bot, chatID, resultsText.String())
}

// --- File handling and message functions remain the same ---

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
		log.Printf("Error sending message: %v", err)
}
}

@@ -120,58 +172,31 @@ func writeDomains(domains []string) error {

func addDomain(domain string) string {
if domain == "" { return "Usage: /add example.com" }
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	domains, _ := readDomains()
for _, d := range domains {
if d == domain { return fmt.Sprintf("Domain '%s' is already in the list.", domain) }
}
	if err := writeDomains(append(domains, domain)); err != nil {
		return "Error writing to domain list."
	}
	domains = append(domains, domain)
	if err := writeDomains(domains); err != nil { return "Error writing to domain list." }
return fmt.Sprintf("✅ Added '%s'", domain)
}

func removeDomain(domain string) string {
if domain == "" { return "Usage: /remove example.com" }
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	domains, _ := readDomains()
var newDomains []string
found := false
for _, d := range domains {
		if d == domain {
			found = true
		} else {
			newDomains = append(newDomains, d)
		}
		if d == domain { found = true; continue }
		newDomains = append(newDomains, d)
}
if !found { return fmt.Sprintf("Domain '%s' not found.", domain) }
if err := writeDomains(newDomains); err != nil { return "Error writing to domain list." }
return fmt.Sprintf("🗑️ Removed '%s'", domain)
}

func listDomains() string {
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	domains, _ := readDomains()
if len(domains) == 0 { return "The domain list is empty." }
return "Domains being checked:\n" + strings.Join(domains, "\n")
}

func checkDomainsAndNotify(bot *tgbotapi.BotAPI, chatID int64) {
	log.Println("Executing checkdomain tool...")
	data, err := os.ReadFile(domainsFilePath)
	if err != nil {
		sendMessage(bot, chatID, "🚨 Error: Could not read persistent domain list.")
		return
	}
	if err := os.WriteFile("./domains.txt", data, 0644); err != nil {
		sendMessage(bot, chatID, "🚨 Error: Could not prepare domains.txt for checker.")
		return
	}
	cmd := exec.Command("./checkdomain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		sendMessage(bot, chatID, fmt.Sprintf("🚨 Domain Check (with error):\n\n```\n%s\nError: %v\n```", string(output), err))
		return
	}
	sendMessage(bot, chatID, fmt.Sprintf("📄 Domain Check Result:\n\n```\n%s\n```", string(output)))
}
