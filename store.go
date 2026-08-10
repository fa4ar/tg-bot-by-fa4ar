package main

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
)

var dataFile = "data.json"
var seedFile = "seed.json"

type Store struct {
	mu   sync.RWMutex
	data Data
}

func NewStore() (*Store, error) {
	s := &Store{}
	if b, err := os.ReadFile(dataFile); err == nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, err
		}
	}
	if err := s.mergeSeed(); err != nil {
		return nil, err
	}
	return s, nil
}

// mergeSeed подхватывает структуру из seed.json (фракции, отряды, колонки гугл-таблицы),
// не трогая уже существующие данные. Либо создаёт её, если файл есть.
func (s *Store) mergeSeed() error {
	b, err := os.ReadFile(seedFile)
	if err != nil {
		return nil
	}
	var seed Data
	if err := json.Unmarshal(b, &seed); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Layouts == nil {
		s.data.Layouts = map[string]SquadLayout{}
	}
	for _, sf := range seed.Factions {
		found := false
		for i := range s.data.Factions {
			if s.data.Factions[i].Name == sf.Name {
				for _, sq := range sf.Squads {
					exists := false
					for _, es := range s.data.Factions[i].Squads {
						if es.Name == sq.Name {
							exists = true
							break
						}
					}
					if !exists {
						s.data.Factions[i].Squads = append(s.data.Factions[i].Squads, sq)
					}
				}
				found = true
				break
			}
		}
		if !found {
			s.data.Factions = append(s.data.Factions, sf)
		}
	}
	for squad, l := range seed.Layouts {
		s.data.Layouts[squad] = l
	}
	return s.save()
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, b, 0o644)
}

func (s *Store) Factions() []Faction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Faction(nil), s.data.Factions...)
}

func (s *Store) FindFaction(name string) *Faction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == name {
			return &s.data.Factions[i]
		}
	}
	return nil
}

func (s *Store) FindSquad(faction, squad string) (*Faction, *Squad) {
	f := s.FindFaction(faction)
	if f == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range f.Squads {
		if f.Squads[i].Name == squad {
			return f, &f.Squads[i]
		}
	}
	return f, nil
}

func (s *Store) FindSquadByName(squad string) (*Faction, *Squad) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Factions {
		for j := range s.data.Factions[i].Squads {
			if s.data.Factions[i].Squads[j].Name == squad {
				return &s.data.Factions[i], &s.data.Factions[i].Squads[j]
			}
		}
	}
	return nil, nil
}

func (s *Store) AddFaction(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FindFaction(name) != nil {
		return errors.New("фракция уже существует")
	}
	s.data.Factions = append(s.data.Factions, Faction{Name: name, Active: true})
	return s.save()
}

func (s *Store) RemoveFaction(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.data.Factions[:0]
	for _, f := range s.data.Factions {
		if f.Name != name {
			out = append(out, f)
		}
	}
	if len(out) == len(s.data.Factions) {
		return errors.New("фракция не найдена")
	}
	s.data.Factions = out
	return s.save()
}

func (s *Store) SetFactionActive(name string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == name {
			s.data.Factions[i].Active = active
			return s.save()
		}
	}
	return errors.New("фракция не найдена")
}

func (s *Store) AddSquad(faction, squad string, size int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == faction {
			for _, sq := range s.data.Factions[i].Squads {
				if sq.Name == squad {
					return errors.New("отряд уже существует")
				}
			}
			s.data.Factions[i].Squads = append(s.data.Factions[i].Squads, Squad{Name: squad, Size: size, Active: true})
			return s.save()
		}
	}
	return errors.New("фракция не найдена")
}

func (s *Store) RemoveSquad(faction, squad string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == faction {
			out := s.data.Factions[i].Squads[:0]
			for _, sq := range s.data.Factions[i].Squads {
				if sq.Name != squad {
					out = append(out, sq)
				}
			}
			if len(out) == len(s.data.Factions[i].Squads) {
				return errors.New("отряд не найден")
			}
			s.data.Factions[i].Squads = out
			return s.save()
		}
	}
	return errors.New("фракция не найдена")
}

func (s *Store) SetSquadActive(faction, squad string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == faction {
			for j := range s.data.Factions[i].Squads {
				if s.data.Factions[i].Squads[j].Name == squad {
					s.data.Factions[i].Squads[j].Active = active
					return s.save()
				}
			}
			return errors.New("отряд не найден")
		}
	}
	return errors.New("фракция не найдена")
}

func (s *Store) AddRole(faction, squad, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == faction {
			for j := range s.data.Factions[i].Squads {
				if s.data.Factions[i].Squads[j].Name == squad {
					for _, r := range s.data.Factions[i].Squads[j].Roles {
						if r.Name == role {
							return errors.New("роль уже существует")
						}
					}
					s.data.Factions[i].Squads[j].Roles = append(s.data.Factions[i].Squads[j].Roles, Role{Name: role})
					return s.save()
				}
			}
			return errors.New("отряд не найден")
		}
	}
	return errors.New("фракция не найдена")
}

func (s *Store) RemoveRole(faction, squad, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Factions {
		if s.data.Factions[i].Name == faction {
			for j := range s.data.Factions[i].Squads {
				if s.data.Factions[i].Squads[j].Name == squad {
					rr := s.data.Factions[i].Squads[j].Roles
					out := rr[:0]
					for _, r := range rr {
						if r.Name != role {
							out = append(out, r)
						}
					}
					if len(out) == len(rr) {
						return errors.New("роль не найдена")
					}
					s.data.Factions[i].Squads[j].Roles = out
					return s.save()
				}
			}
		}
	}
	return errors.New("не найдено")
}

func (s *Store) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Event(nil), s.data.Events...)
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}

func (s *Store) FindEvent(id string) *Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == id {
			return &s.data.Events[i]
		}
	}
	return nil
}

func (s *Store) FindEventByTitle(title string) *Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Events {
		if s.data.Events[i].Title == title {
			return &s.data.Events[i]
		}
	}
	return nil
}

func (s *Store) AddEvent(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Events = append(s.data.Events, ev)
	return s.save()
}

// DeleteEvent удаляет событие по ID
func (s *Store) DeleteEvent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == id {
			s.data.Events = append(s.data.Events[:i], s.data.Events[i+1:]...)
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

// DeleteEventByTitle удаляет событие по названию
func (s *Store) DeleteEventByTitle(title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].Title == title {
			s.data.Events = append(s.data.Events[:i], s.data.Events[i+1:]...)
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

func (s *Store) SetEventPassword(id, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == id {
			s.data.Events[i].Password = password
			s.data.Events[i].PasswordSent = false
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

// SetEventPasswordByTitle устанавливает пароль по названию события
func (s *Store) SetEventPasswordByTitle(title, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].Title == title {
			s.data.Events[i].Password = password
			s.data.Events[i].PasswordSent = false
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

func (s *Store) AddRegistration(evID string, reg Registration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == evID {
			for _, r := range s.data.Events[i].Registrations {
				if r.UserID == reg.UserID {
					return errors.New("ты уже записан на это событие")
				}
			}
			s.data.Events[i].Registrations = append(s.data.Events[i].Registrations, reg)
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

func (s *Store) CancelRegistration(evID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == evID {
			out := s.data.Events[i].Registrations[:0]
			for _, r := range s.data.Events[i].Registrations {
				if r.UserID != userID {
					out = append(out, r)
				}
			}
			if len(out) == len(s.data.Events[i].Registrations) {
				return errors.New("ты не записан на это событие")
			}
			s.data.Events[i].Registrations = out
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

func (s *Store) MarkPasswordSent(evID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == evID {
			s.data.Events[i].PasswordSent = true
			return s.save()
		}
	}
	return errors.New("событие не найдено")
}

func (s *Store) SquadFree(evID, squad string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == evID {
			_, sq := s.findSquadLocked(squad)
			if sq == nil {
				return 0, errors.New("отряд не найден")
			}
			count := 0
			for _, r := range s.data.Events[i].Registrations {
				if r.Squad == squad {
					count++
				}
			}
			return sq.Size - count, nil
		}
	}
	return 0, errors.New("событие не найдено")
}

func (s *Store) findSquadLocked(squad string) (*Faction, *Squad) {
	for i := range s.data.Factions {
		for j := range s.data.Factions[i].Squads {
			if s.data.Factions[i].Squads[j].Name == squad {
				return &s.data.Factions[i], &s.data.Factions[i].Squads[j]
			}
		}
	}
	return nil, nil
}

func (s *Store) SquadsForEvent(evID string) (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Events {
		if s.data.Events[i].ID == evID {
			counts := map[string]int{}
			for _, r := range s.data.Events[i].Registrations {
				counts[r.Squad]++
			}
			slots := map[string]int{}
			for fi := range s.data.Factions {
				for _, sq := range s.data.Factions[fi].Squads {
					slots[sq.Name] = sq.Size - counts[sq.Name]
				}
			}
			return slots, nil
		}
	}
	return nil, errors.New("событие не найдено")
}

func (s *Store) ensureLayoutsLocked() {
	if s.data.Layouts == nil {
		s.data.Layouts = map[string]SquadLayout{}
	}
}

func (s *Store) Layouts() map[string]SquadLayout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]SquadLayout{}
	for k, v := range s.data.Layouts {
		out[k] = v
	}
	return out
}

func (s *Store) GetLayout(squad string) (SquadLayout, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.data.Layouts[squad]
	return l, ok
}

func (s *Store) SetLayout(squad string, l SquadLayout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLayoutsLocked()
	s.data.Layouts[squad] = l
	return s.save()
}

func (s *Store) RemoveLayout(squad string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLayoutsLocked()
	if _, ok := s.data.Layouts[squad]; !ok {
		return errors.New("отряд не был привязан")
	}
	delete(s.data.Layouts, squad)
	return s.save()
}
