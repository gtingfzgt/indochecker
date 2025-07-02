// main.go (API version with message chunking)
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.comcom/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

const domainsFilePath = "/data/domains.txt"
const apiBaseURL = "https://check.skiddle.id/"
const maxDomainsPerRequest = 30 // As per the API documentation
const telegramMaxMsgLen = 4096

var fileMutex = &sync.Mutex{}

type DomainStatus struct {
	Blocked bool `json:"blocked"`
}

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	adminChatIDStr := os.Getenv("ADMIN_CHAT_ID")
	adminChatID, _ := strconv.ParseInt(adminChatIDStr, 10, 64)

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = false // Set to false to reduce log spam
	log.Printf("Authorized on account %s", bot.Self.UserName)

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

	sendMessage(bot, adminChatID, "✅ Bot (re)started! Using public API. Scheduled checks are active.")

	u := tgbotapi.NewUpdate(0)
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil || update.Message.Chat.ID != adminChatID {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		switch update.Message.Command() {
		case "start":
			msg.Text = "Hello! I'm your domain checker bot.\nCommands:\n/add <domain>\n/remove <domain>\n/list\n/checknow"
			sendMessage(bot, adminChatID, msg.Text)
		case "add":
			sendMessage(bot, adminChatID, addDomain(update.Message.CommandArguments()))
		case "remove":
			sendMessage(bot, adminChatID, removeDomain(update.Message.CommandArguments()))
		case "list":
			// *** MODIFIED *** Use chunking for the list command as well
			go sendChunkedMessage(bot, adminChatID, listDomains(), "📋 Domains being checked")
		case "checknow":
			sendMessage(bot, adminChatID, "🚀 Starting manual check via API...")
			go checkDomainsAndNotify(bot, adminChatID)
		default:
			sendMessage(bot, adminChatID, "I don't know that command.")
		}
	}
}

// *** MODIFIED *** The check function now uses the chunking sender
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

	for i := 0; i < len(domains); i += maxDomainsPerRequest {
		end := i + maxDomainsPerRequest
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		url := fmt.Sprintf("%s?domains=%s&json=true", apiBaseURL, strings.Join(batch, ","))
		httpClient := &http.Client{Timeout: 30 * time.Second} // Increased timeout for larger batches
		resp, err := httpClient.Get(url)
		if err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to check batch: %v\n", err))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to read API response: %v\n", err))
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		var statuses map[string]DomainStatus
		if err := json.Unmarshal(body, &statuses); err != nil {
			resultsText.WriteString(fmt.Sprintf("🚨 Failed to parse API JSON for batch. Response: %s\nError: %v\n", string(body), err))
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

	sendChunkedMessage(bot, chatID, resultsText.String(), "📄 Domain Check Results")
}

// *** NEW FUNCTION *** Splits a long message into multiple smaller ones.
func sendChunkedMessage(bot *tgbotapi.BotAPI, chatID int64, text string, prefix string) {
	if text == "" {
		return
	}
	lines := strings.Split(text, "\n")
	var currentMessage strings.Builder

	fullPrefix := prefix + "\n"
	currentMessage.WriteString(fullPrefix)

	for _, line := range lines {
		if currentMessage.Len()+len(line)+1 > telegramMaxMsgLen {
			sendMessage(bot, chatID, currentMessage.String())
			currentMessage.Reset()
			currentMessage.WriteString(fullPrefix)
		}
		currentMessage.WriteString(line + "\n")
	}
	if currentMessage.Len() > len(fullPrefix) {
		sendMessage(bot, chatID, currentMessage.String())
	}
}


// --- Helper functions ---

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func readDomains() ([]string, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()
	content, err := os.ReadFile(domainsFilePath)
	if err != nil { return nil, err }
	var cleaned []string
	for _, d := range strings.Split(string(content), "\n") {
		if d != "" { cleaned = append(cleaned, d) }
	}
	return cleaned, nil
}

func writeDomains(domains []string) error {
	fileMutex.Lock()
	defer fileMutex.Unlock()
	return os.WriteFile(domainsFilePath, []byte(strings.Join(domains, "\n")), 0644)
}

func addDomain(domain string) string {
	if domain == "" { return "Usage: /add example.com" }
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	for _, d := range domains {
		if d == domain { return fmt.Sprintf("Domain '%s' is already in the list.", domain) }
	}
	domains = append(domains, domain)
	if err := writeDomains(domains); err != nil { return "Error writing to domain list." }
	return fmt.Sprintf("✅ Added '%s'", domain)
}

func removeDomain(domain string) string {
	if domain == "" { return "Usage: /remove example.com" }
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	var newDomains []string
	found := false
	for _, d := range domains {
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
	if len(domains) == 0 { return "The domain list is empty." }
	return strings.Join(domains, "\n")
}
