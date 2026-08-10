package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	token := "8949321510:AAHzo0wTfWnP50SD7LexgUHCuhky1Ey7pNU"

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal("Ошибка создания бота:", err)
	}

	bot.Debug = false
	log.Printf("✅ Бот запущен! @%s", bot.Self.UserName)

	setCommands(bot)

	chatsStore := make(map[int64]ChatInfo)
	var mu sync.Mutex

	// ЖЕСТКИЙ ПЕРЕБОР ВСЕХ ЧАТОВ
	log.Println("🔍 Жесткий поиск чатов...")
	forceGetAllChats(bot, chatsStore, &mu)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updateConfig.Limit = 100

	updates := bot.GetUpdatesChan(updateConfig)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for update := range updates {
			if update.CallbackQuery != nil {
				go handleCallback(bot, update.CallbackQuery, chatsStore, &mu)
				continue
			}

			if update.Message == nil {
				continue
			}

			chat := update.Message.Chat
			mu.Lock()
			chatsStore[chat.ID] = ChatInfo{
				ID:       chat.ID,
				Title:    chat.Title,
				Type:     getChatType(chat),
				UserName: chat.UserName,
			}
			mu.Unlock()

			go handleMessage(bot, update.Message, chatsStore, &mu)
		}
	}()

	log.Printf("📊 Найдено чатов: %d", len(chatsStore))
	log.Println("🚀 Бот запущен!")

	<-sigChan
	log.Println("👋 Бот остановлен")
}

type ChatInfo struct {
	ID       int64
	Title    string
	Type     string
	UserName string
}

func getChatType(chat *tgbotapi.Chat) string {
	if chat.IsChannel() {
		return "📢 Канал"
	}
	if chat.IsSuperGroup() {
		return "👥 Супергруппа"
	}
	if chat.IsGroup() {
		return "👥 Группа"
	}
	return "👤 Личный"
}

// ЖЕСТКИЙ ПЕРЕБОР
func forceGetAllChats(bot *tgbotapi.BotAPI, chatsStore map[int64]ChatInfo, mu *sync.Mutex) {
	// Перебираем все update_id от 0 до 1000000
	for offset := 0; offset < 1000000; offset += 100 {
		config := tgbotapi.NewUpdate(offset)
		config.Limit = 100
		updates, err := bot.GetUpdates(config)
		if err != nil {
			log.Printf("❌ Ошибка на offset %d: %v", offset, err)
			break
		}

		if len(updates) == 0 {
			break
		}

		for _, update := range updates {
			mu.Lock()
			// Из сообщений
			if update.Message != nil && update.Message.Chat.ID != 0 {
				chat := update.Message.Chat
				if _, exists := chatsStore[chat.ID]; !exists {
					chatsStore[chat.ID] = ChatInfo{
						ID:       chat.ID,
						Title:    chat.Title,
						Type:     getChatType(chat),
						UserName: chat.UserName,
					}
					log.Printf("✅ Найден: %s - %s", getChatType(chat), chat.Title)
				}
			}

			// Из callback
			if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
				chat := update.CallbackQuery.Message.Chat
				if _, exists := chatsStore[chat.ID]; !exists {
					chatsStore[chat.ID] = ChatInfo{
						ID:       chat.ID,
						Title:    chat.Title,
						Type:     getChatType(chat),
						UserName: chat.UserName,
					}
					log.Printf("✅ Найден (callback): %s - %s", getChatType(chat), chat.Title)
				}
			}

			// Из инлайн запросов
			if update.InlineQuery != nil {
				// инлайн запросы не дают chat_id
			}
			mu.Unlock()
		}

		// Обновляем offset
		offset = updates[len(updates)-1].UpdateID

		// Если обновлений меньше лимита - выходим
		if len(updates) < 100 {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("✅ Сканирование завершено. Найдено чатов: %d", len(chatsStore))
}

func setCommands(bot *tgbotapi.BotAPI) {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "🏠 Главное меню"},
		{Command: "ping", Description: "🏓 Проверка"},
		{Command: "chats", Description: "📋 Список чатов"},
		{Command: "refresh", Description: "🔄 Обновить чаты"},
		{Command: "broadcast", Description: "📢 Рассылка"},
		{Command: "help", Description: "📖 Помощь"},
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	if _, err := bot.Request(config); err != nil {
		log.Printf("⚠️ Ошибка меню: %v", err)
	} else {
		log.Println("✅ Меню команд установлено!")
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, chatsStore map[int64]ChatInfo, mu *sync.Mutex) {
	log.Printf("📩 [%s] %s", message.From.UserName, message.Text)

	switch message.Text {
	case "/start", "/menu":
		sendMainMenu(bot, message.Chat.ID)

	case "/help":
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"📖 *Команды:*\n\n"+
				"/start - 🏠 Меню\n"+
				"/ping - 🏓 Проверка\n"+
				"/chats - 📋 Список чатов\n"+
				"/refresh - 🔄 Обновить чаты\n"+
				"/broadcast [текст] - 📢 Рассылка\n"+
				"/help - 📖 Помощь",
		)
		msg.ParseMode = "Markdown"
		bot.Send(msg)

	case "/ping":
		msg := tgbotapi.NewMessage(message.Chat.ID, "🏓 Pong!")
		bot.Send(msg)

	case "/chats":
		showAllChats(bot, message.Chat.ID, chatsStore, mu)

	case "/refresh":
		msg := tgbotapi.NewMessage(message.Chat.ID, "🔄 Обновляю список чатов...")
		bot.Send(msg)
		forceGetAllChats(bot, chatsStore, mu)
		showAllChats(bot, message.Chat.ID, chatsStore, mu)

	default:
		if strings.HasPrefix(message.Text, "/broadcast") {
			text := strings.TrimSpace(strings.TrimPrefix(message.Text, "/broadcast"))
			if text == "" {
				msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Напиши текст: `/broadcast Всем привет!`")
				msg.ParseMode = "Markdown"
				bot.Send(msg)
				return
			}
			go broadcastMessage(bot, message.Chat.ID, text, message.From.UserName, chatsStore, mu)
		}
	}
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	buttons := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статус", "status"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Инфо", "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Время", "time"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "refresh"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📢 Рассылка", "broadcast_menu"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Чаты", "chats_list"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "🏠 *Главное меню*")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = buttons
	bot.Send(msg)
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, chatsStore map[int64]ChatInfo, mu *sync.Mutex) {
	bot.Send(tgbotapi.NewCallback(callback.ID, ""))

	chatID := callback.Message.Chat.ID
	msgID := callback.Message.MessageID
	data := callback.Data

	var response string

	switch data {
	case "status":
		response = "✅ *Статус:*\n• Работает: ✅\n• Чатов: " + strconv.Itoa(len(chatsStore)) + "\n• Время: " + time.Now().Format("15:04:05")
	case "info":
		response = "📝 *Инфо:*\n• Бот: Telegram Bridge\n• Токен: ✅\n• Чатов: " + strconv.Itoa(len(chatsStore))
	case "time":
		response = "🕐 " + time.Now().Format("02.01.2006 15:04:05")
	case "refresh":
		bot.Send(tgbotapi.NewMessage(chatID, "🔄 Обновляю..."))
		forceGetAllChats(bot, chatsStore, mu)
		showAllChats(bot, chatID, chatsStore, mu)
		return
	case "broadcast_menu":
		response = "📢 Рассылка: `/broadcast Текст`"
	case "chats_list":
		showAllChats(bot, chatID, chatsStore, mu)
		return
	case "back_to_menu":
		sendMainMenu(bot, chatID)
		return
	default:
		response = "❌ Неизвестно"
	}

	backBtn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(chatID, msgID, response)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &backBtn
	bot.Send(edit)
}

func showAllChats(bot *tgbotapi.BotAPI, chatID int64, chatsStore map[int64]ChatInfo, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()

	if len(chatsStore) == 0 {
		msg := tgbotapi.NewMessage(chatID,
			"📭 *Нет чатов*\n\n"+
				"1️⃣ Добавь бота в группу/канал\n"+
				"2️⃣ Напиши боту что-нибудь\n"+
				"3️⃣ Нажми /refresh",
		)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		return
	}

	var private, groups, channels []string

	for _, info := range chatsStore {
		title := info.Title
		if title == "" {
			title = info.UserName
			if title == "" {
				title = "Без имени"
			}
		}
		line := "• `" + strconv.FormatInt(info.ID, 10) + "` - " + title

		if strings.Contains(info.Type, "Канал") {
			channels = append(channels, line)
		} else if strings.Contains(info.Type, "Группа") || strings.Contains(info.Type, "Супер") {
			groups = append(groups, line)
		} else {
			private = append(private, line)
		}
	}

	res := "📋 *Список чатов:*\n\n"

	if len(private) > 0 {
		res += "👤 *Личные (" + strconv.Itoa(len(private)) + "):*\n" + strings.Join(private, "\n") + "\n\n"
	}
	if len(groups) > 0 {
		res += "👥 *Группы (" + strconv.Itoa(len(groups)) + "):*\n" + strings.Join(groups, "\n") + "\n\n"
	}
	if len(channels) > 0 {
		res += "📢 *Каналы (" + strconv.Itoa(len(channels)) + "):*\n" + strings.Join(channels, "\n") + "\n"
	}

	if len(res) > 4000 {
		res = res[:4000] + "\n\n... (обрезано)"
	}

	msg := tgbotapi.NewMessage(chatID, res)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func broadcastMessage(bot *tgbotapi.BotAPI, senderChatID int64, text string, senderName string, chatsStore map[int64]ChatInfo, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()

	log.Printf("📢 Рассылка от %s: %s", senderName, text)

	if len(chatsStore) == 0 {
		bot.Send(tgbotapi.NewMessage(senderChatID, "❌ Нет чатов для рассылки"))
		return
	}

	bot.Send(tgbotapi.NewMessage(senderChatID, "📢 Начинаю рассылку...\n📊 Чатов: "+strconv.Itoa(len(chatsStore))))

	success := 0
	fail := 0
	msgText := "📢 *Рассылка*\n\n" + text + "\n\n👤 От: " + senderName

	for chatID, info := range chatsStore {
		if chatID == senderChatID {
			continue
		}

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"

		if _, err := bot.Send(msg); err != nil {
			fail++
			log.Printf("❌ Ошибка в %s %d: %v", info.Type, chatID, err)
		} else {
			success++
			log.Printf("✅ Отправлено в %s %d", info.Type, chatID)
		}
		time.Sleep(50 * time.Millisecond)
	}

	bot.Send(tgbotapi.NewMessage(senderChatID,
		"✅ *Рассылка завершена!*\n\n"+
			"✅ Успешно: "+strconv.Itoa(success)+"\n"+
			"❌ Ошибок: "+strconv.Itoa(fail),
	))
}
