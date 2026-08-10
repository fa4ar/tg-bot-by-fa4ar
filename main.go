package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
)

func main() {
	cfg := LoadConfig()
	if cfg.Token == "" {
		log.Fatal("задай переменную окружения DISCORD_TOKEN")
	}
	if len(cfg.AdminIDs) == 0 {
		log.Fatal("задай переменную окружения ADMIN_IDS (через запятую твой Discord ID)")
	}

	store, err := NewStore()
	if err != nil {
		log.Fatalf("не удалось загрузить хранилище: %v", err)
	}

	sess, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("ошибка создания сессии: %v", err)
	}
	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	bot := NewBot(sess, store, cfg)
	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		bot.OnInteraction(s, i)
	})

	if err := sess.Open(); err != nil {
		log.Fatalf("не удалось подключиться к Discord: %v", err)
	}
	defer sess.Close()

	if err := bot.RegisterCommands(); err != nil {
		log.Printf("внимание: %v", err)
	}

	go bot.RunScheduler()

	appID := sess.State.User.ID
	invite := fmt.Sprintf("https://discord.com/api/oauth2/authorize?client_id=%s&permissions=0&scope=bot%%20applications.commands", appID)
	log.Printf("бот запущен: %s", sess.State.User.Username)
	log.Printf("инвайт-ссылка (обязательно со scope applications.commands): %s", invite)
	if cfg.GuildID == "" {
		log.Printf("внимание: команды глобальные и могут появиться в Discord с задержкой до 1 часа. Задай GUILD_ID в .env, чтобы команды появились мгновенно.")
	}
	wait := make(chan os.Signal, 1)
	<-wait
}
