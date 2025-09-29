package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryMessage represents a single turn in a conversation
type MemoryMessage struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// MemoryStore persists chat histories per chat JID and assistant name
type MemoryStore struct {
	mu         sync.RWMutex
	FilePath   string
	Data       map[string][]MemoryMessage
	MaxPerChat int
}

// MemStore is the global memory store instance
var MemStore *MemoryStore

// InitMemory initializes the global memory store from a JSON file
func InitMemory(filePath string) error {
	if filePath == "" {
		filePath = "memory.json"
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	store := &MemoryStore{
		FilePath:   filePath,
		Data:       make(map[string][]MemoryMessage),
		MaxPerChat: 50,
	}

	// Load existing if present
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

// GetHistory returns up to limit most recent messages
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

// Append adds a message and trims per-chat history
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

// Save writes the memory store to disk
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

// AppendAndSave is a convenience method to append and persist
func (s *MemoryStore) AppendAndSave(chatJID, assistantName, role, text string) {
	s.Append(chatJID, assistantName, role, text)
	_ = s.Save()
}
