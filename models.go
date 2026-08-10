package main

import "time"

type Role struct {
	Name string `json:"name"`
}

type Squad struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	Active bool   `json:"active"`
	Roles  []Role `json:"roles"`
}

type Faction struct {
	Name   string  `json:"name"`
	Color  string  `json:"color"`
	Active bool    `json:"active"`
	Squads []Squad `json:"squads"`
}

type Registration struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Faction  string `json:"faction"`
	Squad    string `json:"squad"`
	Role     string `json:"role"`
	ArmaID   string `json:"arma_id"`
	Row      int    `json:"row"`
	JoinedAt string `json:"joined_at"`
}

type Event struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	ServerName    string         `json:"server_name"`
	Password      string         `json:"password"`
	StartsAt      time.Time      `json:"starts_at"`
	PasswordSent  bool           `json:"password_sent"`
	Registrations []Registration `json:"registrations"`
}

// SquadLayout — привязка отряда к колонкам и диапазону строк в гугл-таблице.
// Ничего в таблице не создаётся: только адрес, куда писать.
type SquadLayout struct {
	Faction    string `json:"faction"`
	RoleCol    string `json:"role_col"`
	ArmaCol    string `json:"arma_col"`
	DiscordCol string `json:"discord_col"`
	StartRow   int    `json:"start_row"`
	EndRow     int    `json:"end_row"`
}

type Data struct {
	Factions []Faction              `json:"factions"`
	Events   []Event                `json:"events"`
	Layouts  map[string]SquadLayout `json:"layouts"`
}
