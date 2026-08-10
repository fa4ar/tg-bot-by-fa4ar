package main

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Token          string
	AdminIDs       map[string]bool
	TimeZone       string
	GuildID        string
	SheetsCredFile string
	SpreadsheetID  string
}

func LoadConfig() Config {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("предупреждение: .env: %v", err)
	}
	admins := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("ADMIN_IDS"), ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			admins[id] = true
		}
	}
	tz := os.Getenv("TZ")
	if tz == "" {
		tz = "Europe/Moscow"
	}
	return Config{
		Token:          os.Getenv("DISCORD_TOKEN"),
		AdminIDs:       admins,
		TimeZone:       tz,
		GuildID:        os.Getenv("GUILD_ID"),
		SheetsCredFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		SpreadsheetID:  os.Getenv("SPREADSHEET_ID"),
	}
}

func (c Config) IsAdmin(userID string) bool {
	if len(c.AdminIDs) == 0 {
		return false
	}
	return c.AdminIDs[userID]
}
