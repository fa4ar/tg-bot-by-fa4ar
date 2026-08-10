package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
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

	// ============================================
	// УСТАНАВЛИВАЕМ МЕНЮ КОМАНД
	// ============================================
	setCommands(bot)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for update := range updates {
			if update.CallbackQuery != nil {
				handleCallback(bot, update.CallbackQuery)
				continue
			}

			if update.Message == nil {
				continue
			}

			handleMessage(bot, update.Message)
		}
	}()

	log.Println("🚀 Бот запущен!")
	log.Println("📌 Нажми Ctrl+C для остановки")
	<-sigChan
	log.Println("👋 Бот остановлен")
}

// ============================================
// НАСТРОЙКА МЕНЮ КОМАНД
// ============================================
func setCommands(bot *tgbotapi.BotAPI) {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "🏠 Главное меню"},
		{Command: "menu", Description: "📋 Показать меню"},
		{Command: "ping", Description: "🏓 Проверка связи"},
		{Command: "chats", Description: "📋 Список чатов с ботом"},
		{Command: "broadcast", Description: "📢 Рассылка во все чаты"},
		{Command: "help", Description: "📖 Помощь"},
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	_, err := bot.Request(config)
	if err != nil {
		log.Printf("⚠️ Ошибка установки меню команд: %v", err)
	} else {
		log.Println("✅ Меню команд установлено!")
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	log.Printf("📩 [%s] %s: %s",
		message.From.UserName,
		message.From.FirstName,
		message.Text,
	)

	switch {
	case message.Text == "/start":
		sendMainMenu(bot, message.Chat.ID)

	case message.Text == "/menu":
		sendMainMenu(bot, message.Chat.ID)

	case message.Text == "/help":
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"📖 *Команды бота:*\n\n"+
				"/start - 🏠 Главное меню\n"+
				"/menu - 📋 Показать меню\n"+
				"/ping - 🏓 Проверка связи\n"+
				"/chats - 📋 Список чатов с ботом\n"+
				"/broadcast [текст] - 📢 Рассылка во все чаты\n"+
				"/help - 📖 Помощь",
		)
		msg.ParseMode = "Markdown"
		bot.Send(msg)

	case message.Text == "/ping":
		msg := tgbotapi.NewMessage(message.Chat.ID, "🏓 Pong!")
		bot.Send(msg)

	case message.Text == "/chats":
		showAllChats(bot, message.Chat.ID)

	case strings.HasPrefix(message.Text, "/broadcast"):
		text := strings.TrimPrefix(message.Text, "/broadcast")
		text = strings.TrimSpace(text)

		if text == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"❌ Напиши текст после команды!\n\n"+
					"Пример: `/broadcast Всем привет!`",
			)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
			return
		}

		go broadcastMessage(bot, message.Chat.ID, text, message.From.UserName)

	default:
		// Игнорируем все остальные сообщения
		log.Printf("⏭️ Игнорирую: %s", message.Text)
	}
}

func broadcastMessage(bot *tgbotapi.BotAPI, senderChatID int64, text string, senderName string) {
	log.Printf("📢 Начинаем рассылку от %s: %s", senderName, text)

	updates, err := bot.GetUpdates(tgbotapi.NewUpdate(0))
	if err != nil {
		log.Println("❌ Ошибка получения обновлений:", err)
		bot.Send(tgbotapi.NewMessage(senderChatID, "❌ Ошибка получения списка чатов"))
		return
	}

	chatIDs := make(map[int64]bool)
	for _, update := range updates {
		if update.Message != nil {
			chatIDs[update.Message.Chat.ID] = true
		}
		if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
			chatIDs[update.CallbackQuery.Message.Chat.ID] = true
		}
	}

	if len(chatIDs) == 0 {
		bot.Send(tgbotapi.NewMessage(senderChatID, "❌ Нет активных чатов для рассылки"))
		return
	}

	startMsg := tgbotapi.NewMessage(senderChatID,
		"📢 Начинаю рассылку...\n"+
			"📊 Найдено чатов: "+itoa(len(chatIDs)),
	)
	bot.Send(startMsg)

	successCount := 0
	failCount := 0

	messageText := "📢 *Массовое сообщение*\n\n" + text + "\n\n" +
		"━━━━━━━━━━━━━━━━\n" +
		"📨 Отправлено через бота\n" +
		"👤 От: " + senderName

	for chatID := range chatIDs {
		if chatID == senderChatID {
			continue
		}

		msg := tgbotapi.NewMessage(chatID, messageText)
		msg.ParseMode = "Markdown"

		_, err := bot.Send(msg)
		if err != nil {
			failCount++
			log.Printf("❌ Не удалось отправить в чат %d: %v", chatID, err)
		} else {
			successCount++
			log.Printf("✅ Отправлено в чат %d", chatID)
		}

		time.Sleep(100 * time.Millisecond)
	}

	resultMsg := tgbotapi.NewMessage(senderChatID,
		"✅ *Рассылка завершена!*\n\n"+
			"📊 Статистика:\n"+
			"• Всего чатов: "+itoa(len(chatIDs))+"\n"+
			"• Успешно: "+itoa(successCount)+"\n"+
			"• Ошибок: "+itoa(failCount)+"\n\n"+
			"📝 Текст: "+text,
	)
	resultMsg.ParseMode = "Markdown"
	bot.Send(resultMsg)
}

func showAllChats(bot *tgbotapi.BotAPI, chatID int64) {
	updates, err := bot.GetUpdates(tgbotapi.NewUpdate(0))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка получения списка чатов"))
		return
	}

	chatIDs := make(map[int64]string)
	for _, update := range updates {
		if update.Message != nil {
			chat := update.Message.Chat
			chatIDs[chat.ID] = chat.Title
		}
		if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
			chat := update.CallbackQuery.Message.Chat
			chatIDs[chat.ID] = chat.Title
		}
	}

	if len(chatIDs) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "📭 Нет активных чатов"))
		return
	}

	response := "📋 *Список чатов:*\n\n"
	i := 1
	for id, name := range chatIDs {
		if name == "" {
			name = "Личный чат"
		}
		response += itoa(i) + ". `" + itoa(int(id)) + "` - " + name + "\n"
		i++
	}

	msg := tgbotapi.NewMessage(chatID, response)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	var buttons [][]tgbotapi.InlineKeyboardButton

	row1 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📊 Статус", "status"),
		tgbotapi.NewInlineKeyboardButtonData("📝 Информация", "info"),
	)

	row2 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⏰ Время", "time"),
		tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "refresh"),
	)

	row3 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📤 Отправить в MAX", "send_max"),
		tgbotapi.NewInlineKeyboardButtonData("📥 Получить из MAX", "get_max"),
	)

	row4 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📢 Рассылка", "broadcast_menu"),
		tgbotapi.NewInlineKeyboardButtonData("📋 Список чатов", "chats_list"),
	)

	row5 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonURL("🌐 Открыть MAX", "https://web.max.ru"),
	)

	buttons = append(buttons, row1, row2, row3, row4, row5)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	msg := tgbotapi.NewMessage(chatID,
		"🏠 *Главное меню*\n\n"+
			"Выбери действие:",
	)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	bot.Send(tgbotapi.NewCallback(callback.ID, ""))

	data := callback.Data
	chatID := callback.Message.Chat.ID
	userName := callback.From.UserName

	log.Printf("🔘 [%s] Нажал кнопку: %s", userName, data)

	var response string

	switch data {
	case "status":
		response = "✅ *Статус бота:*\n" +
			"• Работает: ✅\n" +
			"• Версия: 2.0.0\n" +
			"• Время: " + time.Now().Format("15:04:05")

	case "info":
		response = "📝 *Информация:*\n" +
			"• Бот: MAX → Telegram Bridge\n" +
			"• Токен: ✅ валидный\n" +
			"• Сообщений: 0"

	case "time":
		now := time.Now().Format("02.01.2006 15:04:05")
		response = "🕐 *Текущее время:*\n" + now

	case "refresh":
		response = "🔄 *Обновлено!*\n" + time.Now().Format("15:04:05")

	case "send_max":
		response = "📤 *Отправка в MAX*\n\n⚠️ Функция в разработке"

	case "get_max":
		response = "📥 *Получение из MAX*\n\n⚠️ Функция в разработке"

	case "broadcast_menu":
		response = "📢 *Рассылка*\n\n" +
			"Команда: `/broadcast Текст`\n" +
			"Пример: `/broadcast Всем привет!`"

	case "chats_list":
		showAllChats(bot, chatID)
		return

	case "back_to_menu":
		sendMainMenu(bot, chatID)
		return

	default:
		response = "❌ Неизвестная команда"
	}

	backBtn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад в меню", "back_to_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, response)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &backBtn

	bot.Send(editMsg)
}

func itoa(i int) string {
	return string(rune(i + '0'))
}
