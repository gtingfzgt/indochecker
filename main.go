// main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

const domainsFilePath = "/data/domains.txt" 
var fileMutex = &sync.Mutex{}

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

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true
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
	log.Println("Cron scheduler started. Checks will run every 30 minutes.")
	
	sendMessage(bot, adminChatID, "✅ Bot started successfully! Scheduled checks are active.")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != adminChatID {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		switch update.Message.Command() {
		case "start":
			msg.Text = "Hello! I'm your domain checker bot.\nCommands:\n/add <domain>\n/remove <domain>\n/list\n/checknow"
		case "add":
			msg.Text = addDomain(update.Message.CommandArguments())
		case "remove":
			msg.Text = removeDomain(update.Message.CommandArguments())
		case "list":
			msg.Text = listDomains()
		case "checknow":
			msg.Text = "🚀 Starting manual check..."
			sendMessage(bot, adminChatID, msg.Text)
			go checkDomainsAndNotify(bot, adminChatID)
			continue
		default:
			msg.Text = "I don't know that command."
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
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
	if err := writeDomains(append(domains, domain)); err != nil {
		return "Error writing to domain list."
	}
	return fmt.Sprintf("✅ Added '%s'", domain)
}

func removeDomain(domain string) string {
	if domain == "" { return "Usage: /remove example.com" }
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
	var newDomains []string
	found := false
	for _, d := range domains {
		if d == domain {
			found = true
		} else {
			newDomains = append(newDomains, d)
		}
	}
	if !found { return fmt.Sprintf("Domain '%s' not found.", domain) }
	if err := writeDomains(newDomains); err != nil { return "Error writing to domain list." }
	return fmt.Sprintf("🗑️ Removed '%s'", domain)
}

func listDomains() string {
	domains, err := readDomains()
	if err != nil { return "Error reading domain list." }
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
