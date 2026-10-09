package memory

import (
	"fmt"
	"strings"
	"time"
)

func (s *Snapshot) ensure() {
	if s.Commitments == nil {
		s.Commitments = []Commitment{}
	}
	if s.People == nil {
		s.People = []Person{}
	}
	if s.Preferences == nil {
		s.Preferences = []Preference{}
	}
	if s.Goals == nil {
		s.Goals = []LongTermGoal{}
	}
	if s.Version == 0 {
		s.Version = SchemaVersion
	}
	if s.Source == "" {
		s.Source = "local on-disk memory"
	}
}

func (s *Snapshot) UpsertCommitment(c Commitment) error {
	s.ensure()
	c.ID = strings.TrimSpace(c.ID)
	c.Title = strings.TrimSpace(c.Title)
	if c.ID == "" || c.Title == "" {
		return fmt.Errorf("memory: commitment id and title are required")
	}
	if c.When.IsZero() {
		return fmt.Errorf("memory: commitment when is required")
	}
	for i := range s.Commitments {
		if s.Commitments[i].ID == c.ID {
			s.Commitments[i] = c
			return nil
		}
	}
	s.Commitments = append(s.Commitments, c)
	return nil
}

func (s *Snapshot) DeleteCommitment(id string) bool {
	return deleteByID(&s.Commitments, id, func(c Commitment) string { return c.ID })
}

func (s *Snapshot) UpsertPerson(p Person) error {
	s.ensure()
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" || p.Name == "" {
		return fmt.Errorf("memory: person id and name are required")
	}
	for i := range s.People {
		if s.People[i].ID == p.ID {
			s.People[i] = p
			return nil
		}
	}
	s.People = append(s.People, p)
	return nil
}

func (s *Snapshot) DeletePerson(id string) bool {
	return deleteByID(&s.People, id, func(p Person) string { return p.ID })
}

func (s *Snapshot) UpsertPreference(p Preference) error {
	s.ensure()
	p.Key = strings.TrimSpace(p.Key)
	if p.Key == "" {
		return fmt.Errorf("memory: preference key is required")
	}
	for i := range s.Preferences {
		if s.Preferences[i].Key == p.Key {
			s.Preferences[i] = p
			return nil
		}
	}
	s.Preferences = append(s.Preferences, p)
	return nil
}

func (s *Snapshot) DeletePreference(key string) bool {
	return deleteByID(&s.Preferences, key, func(p Preference) string { return p.Key })
}

func (s *Snapshot) UpsertGoal(g LongTermGoal) error {
	s.ensure()
	g.ID = strings.TrimSpace(g.ID)
	g.Title = strings.TrimSpace(g.Title)
	g.Status = normalizeGoalStatus(g.Status)
	if g.ID == "" || g.Title == "" {
		return fmt.Errorf("memory: goal id and title are required")
	}
	if g.Status == "" {
		return fmt.Errorf("memory: goal status must be active, paused, or done")
	}
	for i := range s.Goals {
		if s.Goals[i].ID == g.ID {
			s.Goals[i] = g
			return nil
		}
	}
	s.Goals = append(s.Goals, g)
	return nil
}

func (s *Snapshot) DeleteGoal(id string) bool {
	return deleteByID(&s.Goals, id, func(g LongTermGoal) string { return g.ID })
}

func (s Snapshot) CommitmentByID(id string) (Commitment, bool) {
	for _, c := range s.Commitments {
		if c.ID == id {
			return c, true
		}
	}
	return Commitment{}, false
}

func (s Snapshot) PersonByID(id string) (Person, bool) {
	for _, p := range s.People {
		if p.ID == id {
			return p, true
		}
	}
	return Person{}, false
}

func (s Snapshot) GoalByID(id string) (LongTermGoal, bool) {
	for _, g := range s.Goals {
		if g.ID == id {
			return g, true
		}
	}
	return LongTermGoal{}, false
}

func (s Snapshot) ActiveGoals() []LongTermGoal {
	out := make([]LongTermGoal, 0)
	for _, g := range s.Goals {
		if g.Status == "active" {
			out = append(out, g)
		}
	}
	return out
}

func (s Snapshot) CommitmentsOn(day time.Time) []Commitment {
	out := make([]Commitment, 0)
	y, m, d := day.Date()
	for _, c := range s.Commitments {
		cy, cm, cd := c.When.In(day.Location()).Date()
		if cy == y && cm == m && cd == d {
			out = append(out, c)
		}
	}
	return out
}

func (s Snapshot) NextCommitment(at time.Time, within time.Duration) (Commitment, bool) {
	limit := at.Add(within)
	var best Commitment
	found := false
	for _, c := range s.Commitments {
		if !c.When.After(at) || !c.When.Before(limit) {
			continue
		}
		if !found || c.When.Before(best.When) {
			best = c
			found = true
		}
	}
	return best, found
}

func (s Snapshot) PersonNamedIn(text string) (Person, bool) {
	lower := strings.ToLower(text)
	for _, p := range s.People {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(name)) {
			return p, true
		}
	}
	return Person{}, false
}

func deleteByID[T any](slice *[]T, id string, idOf func(T) string) bool {
	id = strings.TrimSpace(id)
	if id == "" || slice == nil {
		return false
	}
	dst := (*slice)[:0]
	found := false
	for _, item := range *slice {
		if idOf(item) == id {
			found = true
			continue
		}
		dst = append(dst, item)
	}
	*slice = dst
	return found
}
