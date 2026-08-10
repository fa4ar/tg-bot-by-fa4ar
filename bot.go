package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	sess   *discordgo.Session
	store  *Store
	cfg    Config
	loc    *time.Location
	sheets *SheetsClient
	stopCh chan struct{}
}

func NewBot(s *discordgo.Session, store *Store, cfg Config) *Bot {
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	return &Bot{sess: s, store: store, cfg: cfg, loc: loc, sheets: NewSheetsClient(cfg), stopCh: make(chan struct{})}
}

func (b *Bot) RegisterCommands() error {
	cmds := []*discordgo.ApplicationCommand{
		b.factionCmd(),
		b.squadCmd(),
		b.roleCmd(),
		b.eventCmd(),
		b.signupCmd(),
		b.cancelCmd(),
		b.mineCmd(),
		b.infoCmd(),
		b.eventsCmd(),
		b.sheetCmd(),
		b.armaCmd(),
	}

	guild := ""
	where := "глобально"
	if b.cfg.GuildID != "" {
		guild = b.cfg.GuildID
		where = "на сервере " + guild
	}

	existing, err := b.sess.ApplicationCommands(b.sess.State.User.ID, guild)
	if err != nil {
		return fmt.Errorf("не удалось получить команды: %w", err)
	}
	existingByName := map[string]*discordgo.ApplicationCommand{}
	for _, c := range existing {
		existingByName[c.Name] = c
	}

	for _, c := range cmds {
		if old, ok := existingByName[c.Name]; ok {
			if _, err := b.sess.ApplicationCommandEdit(b.sess.State.User.ID, guild, old.ID, c); err != nil {
				return fmt.Errorf("не удалось обновить %s: %w", c.Name, err)
			}
			log.Printf("команда обновлена: /%s (%s)", c.Name, where)
		} else {
			if _, err := b.sess.ApplicationCommandCreate(b.sess.State.User.ID, guild, c); err != nil {
				return fmt.Errorf("не удалось создать %s: %w", c.Name, err)
			}
			log.Printf("команда создана: /%s (%s)", c.Name, where)
		}
	}
	return nil
}

func (b *Bot) OnInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
		b.onAutocomplete(s, i)
		return
	}
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	var errMsg string
	switch data.Name {
	case "faction":
		errMsg = b.handleFaction(i)
	case "squad":
		errMsg = b.handleSquad(i)
	case "role":
		errMsg = b.handleRole(i)
	case "event":
		errMsg = b.handleEvent(s, i)
	case "signup":
		errMsg = b.handleSignup(i)
	case "cancel":
		errMsg = b.handleCancel(i)
	case "mine":
		errMsg = b.handleMine(i)
	case "info":
		errMsg = b.handleInfo(i)
	case "events":
		errMsg = b.handleEvents(i)
	case "sheet":
		errMsg = b.handleSheet(i)
	case "arma":
		errMsg = b.handleArma(i)
	}

	if errMsg != "" {
		b.respond(i, errMsg)
	}
}

func (b *Bot) sub(name, desc string, opts ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: desc,
		Options:     opts,
	}
}

func (b *Bot) strOpt(name, desc string, required bool, autocomplete bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         name,
		Description:  desc,
		Required:     required,
		Autocomplete: autocomplete,
	}
}

func (b *Bot) boolOpt(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        name,
		Description: desc,
		Required:    required,
	}
}

func (b *Bot) intOpt(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        name,
		Description: desc,
		Required:    required,
	}
}

func (b *Bot) factionCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "faction",
		Description: "Управление фракциями (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("add", "Добавить фракцию", b.strOpt("name", "Название фракции", true, false)),
			b.sub("remove", "Удалить фракцию", b.strOpt("name", "Название фракции", true, false)),
			b.sub("active", "Включить/выключить фракцию", b.strOpt("name", "Название фракции", true, false), b.boolOpt("status", "true = активна", true)),
		},
	}
}

func (b *Bot) squadCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "squad",
		Description: "Управление отрядами (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("add", "Добавить отряд", b.strOpt("faction", "Название фракции", true, false), b.strOpt("name", "Название отряда", true, false), b.intOpt("size", "Макс. кол-во человек", true)),
			b.sub("remove", "Удалить отряд", b.strOpt("faction", "Название фракции", true, false), b.strOpt("name", "Название отряда", true, false)),
			b.sub("active", "Включить/выключить отряд", b.strOpt("faction", "Название фракции", true, false), b.strOpt("name", "Название отряда", true, false), b.boolOpt("status", "true = активен", true)),
		},
	}
}

func (b *Bot) roleCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "role",
		Description: "Управление ролями в отрядах (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("add", "Добавить роль", b.strOpt("faction", "Название фракции", true, false), b.strOpt("squad", "Название отряда", true, false), b.strOpt("name", "Название роли (снайпер, пулемётчик...)", true, false)),
			b.sub("remove", "Удалить роль", b.strOpt("faction", "Название фракции", true, false), b.strOpt("squad", "Название отряда", true, false), b.strOpt("name", "Название роли", true, false)),
		},
	}
}

func (b *Bot) eventCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "event",
		Description: "Управление ивентами (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("create", "Создать ивент", b.strOpt("name", "Название ивента", true, false), b.strOpt("date", "Дата дд.мм.гггг", true, false), b.strOpt("time", "Время чч:мм", true, false), b.strOpt("server", "Название сервера", false, false)),
			b.sub("password", "Задать пароль сервера", b.strOpt("name", "Название ивента", true, false), b.strOpt("value", "Пароль для доступа", true, false)),
			b.sub("send", "Разослать пароль всем записанным", b.strOpt("name", "Название ивента", true, false)),
			b.sub("list", "Список ивентов"),
			b.sub("delete", "Удалить ивент", b.strOpt("name", "Название ивента", true, false)),
		},
	}
}

func (b *Bot) signupCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "signup",
		Description: "Записаться на ивент",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("join", "Записаться", b.strOpt("event", "Название ивента", true, true), b.strOpt("squad", "Название отряда", true, true), b.strOpt("role", "Роль", true, true), b.strOpt("nik_arma", "Игровой ник в Arma", true, false), b.strOpt("arma_id", "Твой ID Arma Reforger (виден только админу)", true, false)),
		},
	}
}

func (b *Bot) cancelCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "cancel",
		Description: "Отменить запись на ивент",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("me", "Отменить запись", b.strOpt("event", "Название ивента", true, true)),
		},
	}
}

func (b *Bot) mineCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "mine",
		Description: "Мои записи на ивенты",
	}
}

func (b *Bot) infoCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "info",
		Description: "Список фракций, отрядов, ролей и свободных мест",
	}
}

func (b *Bot) eventsCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "events",
		Description: "Список ближайших ивентов",
	}
}

func (b *Bot) sheetCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "sheet",
		Description: "Привязка отрядов к колонкам гугл-таблицы (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("link", "Привязать отряд к колонкам (end можно не указывать)", b.strOpt("squad", "Название отряда", true, true),
				b.strOpt("faction", "Название фракции", true, false),
				b.strOpt("role_col", "Колонка ролей (например A)", true, false),
				b.strOpt("arma_col", "Колонка ник-арма (например B)", true, false),
				b.strOpt("discord_col", "Колонка ник-дискорд (например C)", true, false),
				b.intOpt("start", "Первая строка данных", true),
				b.intOpt("end", "Последняя строка (необязательно — найдётся сам)", false)),
			b.sub("unlink", "Отвязать отряд", b.strOpt("squad", "Название отряда", true, true)),
			b.sub("list", "Список привязок"),
			b.sub("status", "Статус подключения к Google Sheets"),
		},
	}
}

// ---------- обработчики ----------

type dataOpt = *discordgo.ApplicationCommandInteractionDataOption

func authorID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func authorName(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.Username
	}
	if i.User != nil {
		return i.User.Username
	}
	return "unknown"
}

func optString(o []dataOpt) string {
	for _, opt := range o {
		if v, ok := opt.Value.(string); ok {
			return v
		}
	}
	return ""
}

func optBool(o []dataOpt) bool {
	for _, opt := range o {
		if v, ok := opt.Value.(bool); ok {
			return v
		}
	}
	return false
}

func optInt(o []dataOpt) int {
	for _, opt := range o {
		if v, ok := opt.Value.(float64); ok {
			return int(v)
		}
	}
	return 0
}

func optBy(o []dataOpt, name string) string {
	for _, opt := range o {
		if opt.Name == name {
			if v, ok := opt.Value.(string); ok {
				return v
			}
		}
	}
	return ""
}

func (b *Bot) handleFaction(i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	sub := i.ApplicationCommandData().Options[0]
	name := optBy(sub.Options, "name")
	var err error
	switch sub.Name {
	case "add":
		err = b.store.AddFaction(name)
	case "remove":
		err = b.store.RemoveFaction(name)
	case "active":
		err = b.store.SetFactionActive(name, optBool(sub.Options))
	}
	if err != nil {
		return "❌ " + err.Error()
	}
	return "✅ OK: фракция «" + name + "» — " + sub.Name
}

func (b *Bot) handleSquad(i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	sub := i.ApplicationCommandData().Options[0]
	faction := optBy(sub.Options, "faction")
	name := optBy(sub.Options, "name")
	size := optInt(sub.Options)
	var err error
	switch sub.Name {
	case "add":
		err = b.store.AddSquad(faction, name, size)
		if err == nil {
			return fmt.Sprintf("✅ Отряд «%s» во фракции «%s» создан (лимит %d чел.)", name, faction, size)
		}
	case "remove":
		err = b.store.RemoveSquad(faction, name)
	case "active":
		err = b.store.SetSquadActive(faction, name, optBool(sub.Options))
	}
	if err != nil {
		return "❌ " + err.Error()
	}
	return "✅ OK: отряд «" + name + "» — " + sub.Name
}

func (b *Bot) handleRole(i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	sub := i.ApplicationCommandData().Options[0]
	faction := optBy(sub.Options, "faction")
	squad := optBy(sub.Options, "squad")
	name := optBy(sub.Options, "name")
	var err error
	switch sub.Name {
	case "add":
		err = b.store.AddRole(faction, squad, name)
	case "remove":
		err = b.store.RemoveRole(faction, squad, name)
	}
	if err != nil {
		return "❌ " + err.Error()
	}
	return "✅ OK: роль «" + name + "» в отряде «" + squad + "» — " + sub.Name
}

func (b *Bot) handleEvent(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	sub := i.ApplicationCommandData().Options[0]
	vals := map[string]string{}
	for _, o := range sub.Options {
		if v, ok := o.Value.(string); ok {
			vals[o.Name] = v
		}
	}

	switch sub.Name {
	case "create":
		t, err := time.ParseInLocation("02.01.2006 15:04", vals["date"]+" "+vals["time"], b.loc)
		if err != nil {
			return "❌ Неверная дата/время. Формат: дата дд.мм.гггг, время чч:мм"
		}
		ev := Event{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			Title:      vals["name"],
			ServerName: vals["server"],
			StartsAt:   t,
		}
		if err := b.store.AddEvent(ev); err != nil {
			return "❌ " + err.Error()
		}
		return fmt.Sprintf("✅ Ивент «%s» создан — %s (МСК)", vals["name"], t.Format("02.01.2006 15:04"))
	case "password":
		ev := b.store.FindEventByTitle(vals["name"])
		if ev == nil {
			return "❌ Ивент не найден"
		}
		err := b.store.SetEventPassword(ev.ID, vals["value"])
		if err != nil {
			return "❌ " + err.Error()
		}
		return "✅ Пароль для ивента «" + vals["name"] + "» сохранён. Разослать: /event send " + vals["name"]
	case "send":
		ev := b.store.FindEventByTitle(vals["name"])
		if ev == nil {
			return "❌ Ивент не найден"
		}
		if ev.Password == "" {
			return "❌ Сначала задай пароль: /event password"
		}
		if len(ev.Registrations) == 0 {
			return "⚠️ На ивент никто не записан."
		}
		n := b.broadcastPassword(ev)
		if err := b.store.MarkPasswordSent(ev.ID); err != nil {
			log.Printf("ошибка отметки отправки пароля: %v", err)
		}
		return fmt.Sprintf("📨 Пароль разослан %d игрокам из %d записанных.", n, len(ev.Registrations))
	case "list":
		return b.handleEvents(i)
	case "delete":
		ev := b.store.FindEventByTitle(vals["name"])
		if ev == nil {
			return "❌ Ивент не найден"
		}
		err := b.store.DeleteEvent(ev.ID)
		if err != nil {
			return "❌ " + err.Error()
		}
		return "✅ Ивент «" + vals["name"] + "» удалён"
	default:
		return "❌ Неизвестная подкоманда"
	}
}

func (b *Bot) handleSignup(i *discordgo.InteractionCreate) string {
	sub := i.ApplicationCommandData().Options[0]
	vals := map[string]string{}
	for _, o := range sub.Options {
		if v, ok := o.Value.(string); ok {
			vals[o.Name] = v
		}
	}
	eventName, squad, role := vals["event"], vals["squad"], vals["role"]
	armaID, armaNick := vals["arma_id"], vals["nik_arma"]

	if eventName == "" || squad == "" || role == "" {
		return "❌ Не все обязательные поля заполнены"
	}
	if b.sheets.Enabled() {
		if _, ok := b.store.GetLayout(squad); !ok {
			return "❌ Отряд «" + squad + "» не привязан к таблице. Попроси админа: /sheet link"
		}
	}

	ev := b.store.FindEventByTitle(eventName)
	if ev == nil {
		return "❌ Ивент «" + eventName + "» не найден. Список: /events"
	}
	_, sq := b.store.FindSquadByName(squad)
	if sq == nil {
		return "❌ Отряд «" + squad + "» не найден. Список: /info"
	}
	if !sq.Active {
		return "❌ Отряд «" + squad + "» сейчас неактивен."
	}

	rowID := 0
	if b.sheets.Enabled() {
		sheetRow, serr := b.sheets.NextFreeRow(b.store, squad, role)
		if serr != nil {
			return "❌ " + serr.Error()
		}
		if sheetRow <= 0 {
			return "❌ Все слоты роли «" + role + "» уже заняты."
		}
		rowID = sheetRow
	} else {
		roleOK := false
		for _, r := range sq.Roles {
			if r.Name == role {
				roleOK = true
				break
			}
		}
		if !roleOK {
			return "❌ Роль «" + role + "» не найдена в отряде «" + squad + "». Роли: /info"
		}
	}

	free, err := b.store.SquadFree(ev.ID, squad)
	if err != nil {
		return "❌ " + err.Error()
	}
	if free <= 0 {
		return "❌ В отряде «" + squad + "» больше нет мест (лимит " + strconv.Itoa(sq.Size) + ")."
	}
	if armaID == "" {
		return "❌ Укажи свой ID Arma Reforger (arma_id)."
	}
	if armaNick == "" {
		return "❌ Укажи свой игровой ник в Arma (nik_arma)."
	}

	reg := Registration{
		UserID:   authorID(i),
		Username: authorName(i),
		Faction:  sqParentFaction(b.store, squad),
		Squad:    squad,
		Role:     role,
		ArmaID:   armaID,
		Row:      rowID,
		JoinedAt: time.Now().Format("02.01.2006 15:04"),
	}
	if err := b.store.AddRegistration(ev.ID, reg); err != nil {
		return "❌ " + err.Error()
	}
	if b.sheets.Enabled() {
		if err := b.sheets.Fill(b.store, squad, rowID, armaNick, authorName(i)); err != nil {
			_ = b.store.CancelRegistration(ev.ID, authorID(i))
			return "❌ В таблицу записать не удалось, запись отменена: " + err.Error()
		}
	}
	return fmt.Sprintf("✅ Ты записан!\n🏷 Ивент: **%s** (%s)\n🎖 Отряд: **%s** | Роль: **%s**\n🖊 Ник в Arma: %s\n🆔 Arma ID: `%s` (виден только админу)\n\nОсталось мест в отряде: %d",
		ev.Title, ev.StartsAt.Format("02.01.2006 15:04"), squad, role, armaNick, reg.ArmaID, free-1)
}

func sqParentFaction(store *Store, squad string) string {
	f, _ := store.FindSquadByName(squad)
	if f != nil {
		return f.Name
	}
	return ""
}

func (b *Bot) handleCancel(i *discordgo.InteractionCreate) string {
	sub := i.ApplicationCommandData().Options[0]
	name := optBy(sub.Options, "event")
	if name == "" {
		return "❌ Укажите название ивента"
	}

	ev := b.store.FindEventByTitle(name)
	if ev == nil {
		return "❌ Ивент не найден."
	}

	var cancelRow, cancelSquad int
	var cancelSquadName string
	for _, r := range ev.Registrations {
		if r.UserID == authorID(i) {
			cancelSquadName = r.Squad
			cancelRow = r.Row
			cancelSquad = 1
			break
		}
	}

	if cancelSquad == 0 {
		return "❌ Вы не записаны на этот ивент"
	}

	if err := b.store.CancelRegistration(ev.ID, authorID(i)); err != nil {
		return "❌ " + err.Error()
	}

	if b.sheets.Enabled() && cancelSquadName != "" && cancelRow > 0 {
		if err := b.sheets.Clear(b.store, cancelSquadName, cancelRow); err != nil {
			log.Printf("sheets: не удалось очистить ячейку %s: %v", authorID(i), err)
		}
	}
	return "✅ Запись на «" + name + "» отменена."
}

func (b *Bot) handleMine(i *discordgo.InteractionCreate) string {
	evs := b.store.Events()
	var sb strings.Builder
	sb.WriteString("📋 **Мои записи:**\n")
	found := false
	userID := authorID(i)
	for _, ev := range evs {
		for _, r := range ev.Registrations {
			if r.UserID == userID {
				found = true
				free, _ := b.store.SquadFree(ev.ID, r.Squad)
				sb.WriteString(fmt.Sprintf("• **%s** — %s | отряд %s | роль %s | ID %s | мест свободно: %d\n",
					ev.Title, ev.StartsAt.Format("02.01.2006 15:04"), r.Squad, r.Role, r.ArmaID, free))
			}
		}
	}
	if !found {
		return "Ты пока никуда не записан. Запись: /signup"
	}
	return sb.String()
}

func (b *Bot) respond(i *discordgo.InteractionCreate, content string) {
	err := b.sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		log.Printf("respond error: %v", err)
	}
}

func (b *Bot) handleInfo(i *discordgo.InteractionCreate) string {
	return b.buildStatus()
}

func (b *Bot) buildStatus() string {
	factions := b.store.Factions()
	if len(factions) == 0 {
		return "Пока нет фракций. Админ: /faction add"
	}
	var sb strings.Builder
	sb.WriteString("🗺 **Структура сил:**\n\n")
	for _, f := range factions {
		status := "🟢 активно"
		if !f.Active {
			status = "🔴 неактивно"
		}
		sb.WriteString(fmt.Sprintf("**%s** — %s\n", f.Name, status))
		for _, sq := range f.Squads {
			sqStatus := "✅ активен"
			if !sq.Active {
				sqStatus = "❌ неактивен"
			}
			sb.WriteString(fmt.Sprintf("  └ **%s** (лимит %d) — %s\n", sq.Name, sq.Size, sqStatus))

			if b.sheets.Enabled() {
				if _, ok := b.store.GetLayout(sq.Name); ok {
					slots, err := b.sheets.Roles(b.store, sq.Name)
					if err == nil && len(slots) > 0 {
						for _, s := range slots {
							mark := "✅ свободна"
							if s.Occupied {
								mark = "❌ занята"
							}
							sb.WriteString(fmt.Sprintf("      • %s — %s\n", s.Role, mark))
						}
					} else {
						sb.WriteString("      • (не удалось прочитать роли из таблицы)\n")
					}
					continue
				}
			}
			if len(sq.Roles) > 0 {
				for _, r := range sq.Roles {
					sb.WriteString(fmt.Sprintf("      • %s\n", r.Name))
				}
			} else {
				sb.WriteString("      • (нет ролей)\n")
			}
		}
	}
	sb.WriteString("\nСвободные места по ивентам: /events")
	return sb.String()
}

func (b *Bot) handleEvents(i *discordgo.InteractionCreate) string {
	evs := b.store.Events()
	now := time.Now()
	var sb strings.Builder
	sb.WriteString("📅 **Ивенты:**\n")
	shown := false
	for _, ev := range evs {
		if ev.StartsAt.Before(now) && ev.PasswordSent {
			continue
		}
		shown = true
		slots, err := b.store.SquadsForEvent(ev.ID)
		slotsStr := ""
		if err == nil && len(slots) > 0 {
			names := make([]string, 0, len(slots))
			for k, v := range slots {
				if v > 0 {
					names = append(names, fmt.Sprintf("%s: %d мест", k, v))
				}
			}
			sort.Strings(names)
			slotsStr = strings.Join(names, ", ")
		}
		sb.WriteString(fmt.Sprintf("• **%s** — %s | записано: %d\n", ev.Title, ev.StartsAt.Format("02.01.2006 15:04"), len(ev.Registrations)))
		if slotsStr != "" {
			sb.WriteString("    " + slotsStr + "\n")
		}
		if ev.ServerName != "" {
			sb.WriteString("    Сервер: " + ev.ServerName + "\n")
		}
		if ev.Password != "" {
			sb.WriteString("    🔒 Пароль задан\n")
		} else {
			sb.WriteString("    🔓 Пароль ещё не задан\n")
		}
		sb.WriteString("\n")
	}
	if !shown {
		sb.WriteString("Нет ближайших ивентов.\n\n")
	}
	sb.WriteString("Запись: /signup")
	return sb.String()
}

func (b *Bot) broadcastPassword(ev *Event) int {
	dms := 0
	for _, r := range ev.Registrations {
		ch, err := b.sess.UserChannelCreate(r.UserID)
		if err != nil {
			log.Printf("DM open fail %s: %v", r.Username, err)
			continue
		}
		msg := fmt.Sprintf("🔐 **Пароль для доступа на сервер**\n\n"+
			"Ивент: **%s**\nДата: %s\nСервер: %s\nПароль: `%s`\n\nУдачной игры, %s! 🎖",
			ev.Title, ev.StartsAt.Format("02.01.2006 15:04"), ev.ServerName, ev.Password, r.Username)
		_, err = b.sess.ChannelMessageSend(ch.ID, msg)
		if err != nil {
			log.Printf("DM send fail %s: %v", r.Username, err)
		} else {
			dms++
		}
	}
	return dms
}

// ---------- автодополнение ----------

func (b *Bot) onAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 || len(data.Options[0].Options) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	addStr := func(vals []string) {
		for _, v := range vals {
			if v != "" {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: v, Value: v})
			}
		}
	}

	subCmd := data.Options[0]
	var focused string
	for _, o := range subCmd.Options {
		if o.Focused {
			focused = o.Name
			break
		}
	}

	switch {
	case data.Name == "signup" && subCmd.Name == "join":
		switch focused {
		case "event":
			for _, ev := range b.store.Events() {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: ev.Title, Value: ev.Title})
			}
		case "squad":
			var sqs []string
			for _, f := range b.store.Factions() {
				if !f.Active {
					continue
				}
				for _, sq := range f.Squads {
					if sq.Active {
						sqs = append(sqs, sq.Name)
					}
				}
			}
			addStr(sqs)
		case "role":
			var roles []string
			for _, f := range b.store.Factions() {
				for _, sq := range f.Squads {
					if b.sheets.Enabled() {
						if _, ok := b.store.GetLayout(sq.Name); ok {
							slots, err := b.sheets.Roles(b.store, sq.Name)
							if err == nil {
								for _, s := range slots {
									if !s.Occupied {
										roles = append(roles, s.Role)
									}
								}
								continue
							}
						}
					}
					for _, r := range sq.Roles {
						roles = append(roles, r.Name)
					}
				}
			}
			addStr(roles)
		}
	case data.Name == "cancel" && subCmd.Name == "me":
		if focused == "event" {
			for _, ev := range b.store.Events() {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: ev.Title, Value: ev.Title})
			}
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// ---------- планировщик рассылки пароля ----------

func (b *Bot) RunScheduler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case now := <-ticker.C:
			for _, ev := range b.store.Events() {
				if ev.Password == "" || ev.PasswordSent {
					continue
				}
				finish := ev.StartsAt.Add(4 * time.Hour)
				if now.After(ev.StartsAt) && now.Before(finish) {
					n := b.broadcastPassword(&ev)
					if err := b.store.MarkPasswordSent(ev.ID); err == nil {
						log.Printf("пароль разослан для ивента %s (%d DM)", ev.Title, n)
					}
				}
			}
		}
	}
}

func (b *Bot) armaCmd() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "arma",
		Description: "Discord ↔ Arma ID по ивентам (админ)",
		Options: []*discordgo.ApplicationCommandOption{
			b.sub("list", "Все записи со всех ивентов"),
			b.sub("event", "Записи конкретного ивента", b.strOpt("name", "Название ивента", true, true)),
		},
	}
}

func (b *Bot) handleArma(i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	sub := i.ApplicationCommandData().Options[0]
	var sb strings.Builder
	evs := b.store.Events()
	for _, ev := range evs {
		if sub.Name == "event" {
			name := optBy(sub.Options, "name")
			if ev.Title != name {
				continue
			}
		}
		for _, r := range ev.Registrations {
			sb.WriteString(fmt.Sprintf("`@%s` (Discord ID: %s | <@%s>) → **%s** — ивент «%s», отряд %s, роль %s\n",
				r.Username, r.UserID, r.UserID, r.ArmaID, ev.Title, r.Squad, r.Role))
		}
	}
	if sb.Len() == 0 {
		return "Записей с Arma ID пока нет."
	}
	return "🔒 **БД Discord ↔ Arma ID:**\n" + sb.String()
}

func (b *Bot) handleSheet(i *discordgo.InteractionCreate) string {
	if !b.cfg.IsAdmin(authorID(i)) {
		return "⛔ Только админ может использовать эту команду."
	}
	if !b.sheets.Enabled() {
		return "❌ Google Таблица не настроена (задай GOOGLE_CREDENTIALS_FILE и SPREADSHEET_ID в .env)"
	}

	sub := i.ApplicationCommandData().Options[0]
	switch sub.Name {
	case "status":
		return "✅ Google Таблица доступна. Привязи отрядов: /sheet list"
	case "link":
		squad := optBy(sub.Options, "squad")
		faction := optBy(sub.Options, "faction")
		if _, sq := b.store.FindSquadByName(squad); sq == nil {
			return "❌ Отряд «" + squad + "» не найден. Сначала: /squad add"
		}
		layout := SquadLayout{
			Faction:    faction,
			RoleCol:    strings.ToUpper(optBy(sub.Options, "role_col")),
			ArmaCol:    strings.ToUpper(optBy(sub.Options, "arma_col")),
			DiscordCol: strings.ToUpper(optBy(sub.Options, "discord_col")),
			StartRow:   optInt(sub.Options),
			EndRow:     secondInt(sub.Options),
		}
		if layout.RoleCol == "" || layout.ArmaCol == "" || layout.DiscordCol == "" {
			return "❌ Укажи все три колонки (role_col, arma_col, discord_col)"
		}
		if layout.StartRow <= 0 || layout.EndRow < layout.StartRow {
			return "❌ Неверный диапазон строк"
		}
		if err := b.store.SetLayout(squad, layout); err != nil {
			return "❌ " + err.Error()
		}
		// проверим, что роли реально читаются из таблицы
		roles, err := b.sheets.Roles(b.store, squad)
		if err != nil {
			return "⚠️ Привязка сохранена, но роли не прочитались: " + err.Error()
		}
		list := ""
		for _, r := range roles {
			mark := "свободна"
			if r.Occupied {
				mark = "занята"
			}
			list += fmt.Sprintf("\n• %s — %s", r.Role, mark)
		}
		return fmt.Sprintf("✅ Отряд «%s» привязан:\nколонки: роль %s, ник-арма %s, ник-дискорд %s, строки %d–%d\n\nРоли из таблицы:%s",
			squad, layout.RoleCol, layout.ArmaCol, layout.DiscordCol, layout.StartRow, layout.EndRow, list)
	case "unlink":
		squad := optBy(sub.Options, "squad")
		if err := b.store.RemoveLayout(squad); err != nil {
			return "❌ " + err.Error()
		}
		return "✅ Отряд «" + squad + "» отвязан."
	case "list":
		layouts := b.store.Layouts()
		if len(layouts) == 0 {
			return "Привязок нет. Добавить: /sheet link"
		}
		var sb strings.Builder
		sb.WriteString("📌 **Привязка отрядов:**\n")
		names := make([]string, 0, len(layouts))
		for k := range layouts {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, n := range names {
			l := layouts[n]
			sb.WriteString(fmt.Sprintf("• **%s** (фракция %s): роль=%s арма=%s дискорд=%s строки %d–%d\n",
				n, l.Faction, l.RoleCol, l.ArmaCol, l.DiscordCol, l.StartRow, l.EndRow))
		}
		return sb.String()
	default:
		return "❌ Неизвестная подкоманда"
	}
}

func secondInt(o []dataOpt) int {
	seen := false
	for _, opt := range o {
		if v, ok := opt.Value.(float64); ok {
			if seen {
				return int(v)
			}
			seen = true
		}
	}
	return 0
}
