package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/relentlessworks/formkit/internal/model"
)

// Store manages JSON file persistence.
type Store struct {
	mu   sync.RWMutex
	dir  string
	data *dbData
}

type dbData struct {
	Workspaces  map[string]*model.Workspace  `json:"workspaces"`
	Forms       map[string]*model.Form       `json:"forms"`
	Submissions map[string]*model.Submission `json:"submissions"`
	Tokens      map[string]*model.Token      `json:"tokens"`
	OTPs        map[string]*model.OTP        `json:"otps"`
}

// New creates a new store backed by JSON files in the given directory.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{
		dir: dir,
		data: &dbData{
			Workspaces:  make(map[string]*model.Workspace),
			Forms:       make(map[string]*model.Form),
			Submissions: make(map[string]*model.Submission),
			Tokens:      make(map[string]*model.Token),
			OTPs:        make(map[string]*model.OTP),
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) dbPath() string {
	return filepath.Join(s.dir, "formkit.json")
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.dbPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read data file: %w", err)
	}
	return json.Unmarshal(b, s.data)
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}
	return os.WriteFile(s.dbPath(), b, 0644)
}

// --- Workspaces ---

func (s *Store) CreateWorkspace(w *model.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Workspaces[w.Handle] = w
	return s.save()
}

func (s *Store) GetWorkspaceByHandle(handle string) (*model.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.data.Workspaces[handle]
	if !ok {
		return nil, fmt.Errorf("workspace not found")
	}
	return w, nil
}

func (s *Store) GetWorkspaceByEmail(email string) (*model.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, w := range s.data.Workspaces {
		if strings.EqualFold(w.Email, email) {
			return w, nil
		}
	}
	return nil, fmt.Errorf("workspace not found")
}

// --- Forms ---

func (s *Store) CreateForm(f *model.Form) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Forms[f.Handle] = f
	return s.save()
}

func (s *Store) GetForm(handle string) (*model.Form, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.data.Forms[handle]
	if !ok {
		return nil, fmt.Errorf("form not found")
	}
	return f, nil
}

func (s *Store) UpdateForm(f *model.Form) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Forms[f.Handle] = f
	return s.save()
}

func (s *Store) DeleteForm(handle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Forms, handle)
	// Also delete submissions for this form
	for sh, sub := range s.data.Submissions {
		if sub.FormHandle == handle {
			delete(s.data.Submissions, sh)
		}
	}
	return s.save()
}

func (s *Store) ListForms(workspace string) []*model.Form {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var forms []*model.Form
	for _, f := range s.data.Forms {
		if f.Workspace == workspace {
			forms = append(forms, f)
		}
	}
	sort.Slice(forms, func(i, j int) bool {
		return forms[i].CreatedAt.Before(forms[j].CreatedAt)
	})
	return forms
}

// --- Submissions ---

func (s *Store) CreateSubmission(sub *model.Submission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Submissions[sub.Handle] = sub
	// Increment form submission count
	if f, ok := s.data.Forms[sub.FormHandle]; ok {
		f.SubCount++
		f.UpdatedAt = sub.CreatedAt
	}
	return s.save()
}

func (s *Store) GetSubmission(handle string) (*model.Submission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data.Submissions[handle]
	if !ok {
		return nil, fmt.Errorf("submission not found")
	}
	return sub, nil
}

func (s *Store) ListSubmissions(formHandle string) []*model.Submission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var subs []*model.Submission
	for _, sub := range s.data.Submissions {
		if sub.FormHandle == formHandle {
			subs = append(subs, sub)
		}
	}
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].CreatedAt.Before(subs[j].CreatedAt)
	})
	return subs
}

func (s *Store) DeleteSubmission(handle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.data.Submissions[handle]
	if !ok {
		return fmt.Errorf("submission not found")
	}
	delete(s.data.Submissions, handle)
	// Decrement form submission count
	if f, ok := s.data.Forms[sub.FormHandle]; ok && f.SubCount > 0 {
		f.SubCount--
	}
	return s.save()
}

// --- Tokens ---

func (s *Store) SaveToken(t *model.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Tokens[t.Token] = t
	return s.save()
}

func (s *Store) GetToken(token string) (*model.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data.Tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return t, nil
}

func (s *Store) DeleteToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Tokens, token)
	return s.save()
}

// --- OTPs ---

func (s *Store) SaveOTP(o *model.OTP) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.OTPs[o.Email] = o
	return s.save()
}

func (s *Store) GetOTP(email string) (*model.OTP, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.data.OTPs[email]
	if !ok {
		return nil, fmt.Errorf("no OTP found for this email")
	}
	return o, nil
}

func (s *Store) DeleteOTP(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.OTPs, email)
	return s.save()
}
