package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type MemoryMessage struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type MemoryStore struct {
	mu         sync.RWMutex
	FilePath   string
	Data       map[string][]MemoryMessage
	MaxPerChat int
}

var MemStore *MemoryStore

func InitMemory(filePath string) error {
	if filePath == "" {
		filePath = "memory.json"
	}

	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	store := &MemoryStore{
		FilePath:   filePath,
		Data:       make(map[string][]MemoryMessage),
		MaxPerChat: 50,
	}

	if _, err := os.Stat(filePath); err == nil {
		b, err := os.ReadFile(filePath)
		if err == nil && len(b) > 0 {
			_ = json.Unmarshal(b, &store.Data)
		}
	}

	MemStore = store
	return nil
}

func (s *MemoryStore) key(chatJID, assistantName string) string {
	return chatJID + "|" + assistantName
}

func (s *MemoryStore) GetHistory(chatJID, assistantName string, limit int) []MemoryMessage {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.key(chatJID, assistantName)
	h := s.Data[key]
	if limit <= 0 || len(h) <= limit {
		return append([]MemoryMessage(nil), h...)
	}
	return append([]MemoryMessage(nil), h[len(h)-limit:]...)
}

func (s *MemoryStore) Append(chatJID, assistantName, role, text string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(chatJID, assistantName)
	msg := MemoryMessage{Role: role, Text: text, Timestamp: time.Now().Unix()}
	s.Data[key] = append(s.Data[key], msg)
	if s.MaxPerChat > 0 && len(s.Data[key]) > s.MaxPerChat {
		over := len(s.Data[key]) - s.MaxPerChat
		s.Data[key] = s.Data[key][over:]
	}
}

func (s *MemoryStore) Save() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.FilePath, b, 0o644)
}

func (s *MemoryStore) AppendAndSave(chatJID, assistantName, role, text string) {
	s.Append(chatJID, assistantName, role, text)
	_ = s.Save()
}
