package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColHelpers(t *testing.T) {
	if colIndex("A") != 1 || colIndex("J") != 10 || colIndex("AA") != 27 {
		t.Fatalf("colIndex: %d %d %d", colIndex("A"), colIndex("J"), colIndex("AA"))
	}
	if colLetter(1) != "A" || colLetter(10) != "J" || colLetter(27) != "AA" {
		t.Fatalf("colLetter: %s %s %s", colLetter(1), colLetter(10), colLetter(27))
	}
	if !isSkipRole("") || !isSkipRole(" пусто ") || isSkipRole("Стрелок") {
		t.Fatal("isSkipRole не работает")
	}
}

func TestMergeSeed(t *testing.T) {
	tmp := t.TempDir()
	dataFile = filepath.Join(tmp, "data.json")
	seedFile = filepath.Join(tmp, "seed.json")

	seed := `{
		"factions": [{"name":"Армия РФ","active":true,"squads":[{"name":"Гром-20","size":10,"active":true,"roles":[{"name":"Командир"}]}]}],
		"layouts": {"Гром-20": {"faction":"Армия РФ","role_col":"A","arma_col":"B","discord_col":"C","start_row":30,"end_row":41}}
	}`
	if err := os.WriteFile(seedFile, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Factions()) != 1 || len(s.Factions()[0].Squads) != 1 {
		t.Fatalf("фракция не импортирована: %+v", s.Factions())
	}
	l, ok := s.GetLayout("Гром-20")
	if !ok || l.RoleCol != "A" || l.StartRow != 30 || l.EndRow != 41 {
		t.Fatalf("layout не импортирован: %+v ok=%v", l, ok)
	}

	// повторный проход не должен дублировать отряды
	s2, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Factions()[0].Squads) != 1 {
		t.Fatalf("отряд задвоился: %+v", s2.Factions()[0].Squads)
	}
}
