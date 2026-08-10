package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsClient struct {
	svc           *sheets.Service
	spreadsheetID string
	available     bool
}

func NewSheetsClient(cfg Config) *SheetsClient {
	if cfg.SheetsCredFile == "" || cfg.SpreadsheetID == "" {
		return &SheetsClient{}
	}
	ctx := context.Background()
	svc, err := sheets.NewService(ctx,
		option.WithCredentialsFile(cfg.SheetsCredFile),
		option.WithScopes(sheets.SpreadsheetsScope))
	if err != nil {
		log.Printf("google sheets: не удалось создать клиент: %v", err)
		return &SheetsClient{}
	}
	return &SheetsClient{svc: svc, spreadsheetID: cfg.SpreadsheetID, available: true}
}

func (sc *SheetsClient) Enabled() bool { return sc != nil && sc.available }

const maxScanRows = 1000

type RoleSlot struct {
	Role     string
	Row      int
	Occupied bool
}

// isSkipRole — пустые слоты-заглушки в таблице («Пусто» и т.п.) не считаются ролями.
func isSkipRole(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return low == "" || low == "пусто" || low == "пуст" || low == "пустая"
}

func colIndex(letter string) int {
	n := 0
	for _, ch := range strings.ToUpper(letter) {
		n = n*26 + int(ch-'A') + 1
	}
	return n
}

func colLetter(idx int) string {
	s := ""
	for idx > 0 {
		idx--
		s = string(rune('A'+idx%26)) + s
		idx /= 26
	}
	return s
}

// readCols читает несколько колонок из одного диапазона сразу — строки всегда выровнены,
// даже если Google обрезает пустые ячейки. Возвращает map: колонка -> значения по строкам.
func (sc *SheetsClient) readCols(store *Store, squad string, cols []string, start, end int) (map[string][]string, error) {
	if len(cols) == 0 {
		return map[string][]string{}, nil
	}
	minI, maxI := colIndex(cols[0]), colIndex(cols[0])
	for _, c := range cols {
		if i := colIndex(c); i < minI {
			minI = i
		} else if i > maxI {
			maxI = i
		}
	}
	ctx := context.Background()
	rng := fmt.Sprintf("%s%d:%s%d", colLetter(minI), start, colLetter(maxI), end)
	resp, err := sc.svc.Spreadsheets.Values.Get(sc.spreadsheetID, rng).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("google sheets: не удалось прочитать %s: %w", rng, err)
	}
	out := make(map[string][]string, len(cols))
	for _, c := range cols {
		out[c] = make([]string, 0, end-start+1)
	}
	for _, row := range resp.Values {
		for _, c := range cols {
			v := ""
			idx := colIndex(c) - minI
			if idx < len(row) {
				v = fmt.Sprintf("%v", row[idx])
			}
			out[c] = append(out[c], strings.TrimSpace(v))
		}
	}
	for _, c := range cols {
		for len(out[c]) < end-start+1 {
			out[c] = append(out[c], "")
		}
	}
	return out, nil
}

func (sc *SheetsClient) getLayout(store *Store, squad string) (SquadLayout, error) {
	l, ok := store.GetLayout(squad)
	if !ok {
		return l, fmt.Errorf("отряд «%s» не привязан к таблице. Попроси админа: /sheet link", squad)
	}
	return l, nil
}

// AutoEnd находит последнюю заполненную строку в колонке ролей от start вниз.
func (sc *SheetsClient) AutoEnd(store *Store, squad string, start int) (int, error) {
	layout, err := sc.getLayout(store, squad)
	if err != nil {
		return 0, err
	}
	vals, err := sc.readCols(store, squad, []string{layout.RoleCol}, start, start+maxScanRows)
	if err != nil {
		return 0, err
	}
	last := 0
	for i, v := range vals[layout.RoleCol] {
		if !isSkipRole(v) {
			last = i + 1
		}
	}
	if last == 0 {
		return 0, fmt.Errorf("в колонке %s не найдено ни одной роли, начиная со строки %d", layout.RoleCol, start)
	}
	return start + last - 1, nil
}

// Roles возвращает роли отряда прямо из таблицы (они уже вписаны), по строке на каждую.
func (sc *SheetsClient) Roles(store *Store, squad string) ([]RoleSlot, error) {
	if !sc.Enabled() {
		return nil, fmt.Errorf("google таблица не настроена")
	}
	layout, err := sc.getLayout(store, squad)
	if err != nil {
		return nil, err
	}
	data, err := sc.readCols(store, squad,
		[]string{layout.RoleCol, layout.DiscordCol}, layout.StartRow, layout.EndRow)
	if err != nil {
		return nil, err
	}
	roles := data[layout.RoleCol]
	disc := data[layout.DiscordCol]
	slots := make([]RoleSlot, 0, len(roles))
	for i, r := range roles {
		if isSkipRole(r) {
			continue
		}
		occ := false
		if i < len(disc) && !isSkipRole(disc[i]) {
			occ = true
		}
		slots = append(slots, RoleSlot{Role: r, Row: layout.StartRow + i, Occupied: occ})
	}
	return slots, nil
}

// NextFreeRow находит первую свободную строку с нужной ролью.
// Возвращает -1, если роль есть, но все её слоты заняты.
func (sc *SheetsClient) NextFreeRow(store *Store, squad, role string) (int, error) {
	if !sc.Enabled() {
		return 0, fmt.Errorf("google таблица не настроена")
	}
	layout, err := sc.getLayout(store, squad)
	if err != nil {
		return 0, err
	}
	data, err := sc.readCols(store, squad,
		[]string{layout.RoleCol, layout.DiscordCol}, layout.StartRow, layout.EndRow)
	if err != nil {
		return 0, err
	}
	roles := data[layout.RoleCol]
	disc := data[layout.DiscordCol]
	found := false
	for i, r := range roles {
		if isSkipRole(r) {
			continue
		}
		if strings.EqualFold(r, role) {
			found = true
			if i < len(disc) && isSkipRole(disc[i]) {
				return layout.StartRow + i, nil
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("роль «%s» не найдена в таблице отряда «%s». Роли: /info", role, squad)
	}
	return -1, fmt.Errorf("все слоты роли «%s» в отряде «%s» уже заняты", role, squad)
}

// Fill записывает игровой ник (Arma) и никнейм Discord в ячейки указанной строки. Ничего не создаёт.
func (sc *SheetsClient) Fill(store *Store, squad string, row int, armaNick, discordNick string) error {
	layout, err := sc.getLayout(store, squad)
	if err != nil {
		return err
	}
	ctx := context.Background()
	updates := []*sheets.ValueRange{
		{Range: fmt.Sprintf("%s%d", layout.ArmaCol, row), Values: [][]interface{}{{armaNick}}},
		{Range: fmt.Sprintf("%s%d", layout.DiscordCol, row), Values: [][]interface{}{{discordNick}}},
	}
	for _, u := range updates {
		if _, err := sc.svc.Spreadsheets.Values.Update(sc.spreadsheetID, u.Range, u).
			ValueInputOption("USER_ENTERED").Context(ctx).Do(); err != nil {
			return fmt.Errorf("google sheets: не удалось записать %s: %w", u.Range, err)
		}
	}
	return nil
}

// Clear очищает ячейки никнеймов в указанной строке (при отмене записи).
func (sc *SheetsClient) Clear(store *Store, squad string, row int) error {
	layout, err := sc.getLayout(store, squad)
	if err != nil {
		return err
	}
	ctx := context.Background()
	updates := []*sheets.ValueRange{
		{Range: fmt.Sprintf("%s%d", layout.ArmaCol, row), Values: [][]interface{}{{""}}},
		{Range: fmt.Sprintf("%s%d", layout.DiscordCol, row), Values: [][]interface{}{{""}}},
	}
	for _, u := range updates {
		if _, err := sc.svc.Spreadsheets.Values.Update(sc.spreadsheetID, u.Range, u).
			ValueInputOption("USER_ENTERED").Context(ctx).Do(); err != nil {
			return fmt.Errorf("google sheets: не удалось очистить %s: %w", u.Range, err)
		}
	}
	return nil
}
