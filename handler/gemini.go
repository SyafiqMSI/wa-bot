package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Gemini API structures
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// GeminiClient holds the configuration for Gemini API
type GeminiClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient() *GeminiClient {
	apiKey := os.Getenv("API_KEY_GEMINI")
	if apiKey == "" {
		log.Println("warning: API_KEY_GEMINI environment variable not set")
	}

	return &GeminiClient{
		APIKey:  apiKey,
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateResponse sends a message to Gemini and returns the response
func (c *GeminiClient) GenerateResponse(ctx context.Context, message string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("gemini API key not configured")
	}

	// Create personalized prompt for the assistant
	systemPrompt := `Kamu adalah Fiq, asisten pribadi yang cerdas, membantu, dan ramah. 
Kamu dibuat untuk membantu pengguna dengan berbagai hal sehari-hari.
Selalu jawab dalam bahasa Indonesia yang sopan dan mudah dipahami.
Jika ditanya tentang identitasmu, katakan bahwa kamu adalah Fiq, asisten pribadi yang dibuat untuk membantu.
Jangan sebutkan bahwa kamu adalah AI atau bot kecuali ditanya secara spesifik.

Pesan pengguna: `

	fullPrompt := systemPrompt + message

	// Prepare request payload
	requestData := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s?key=%s", c.BaseURL, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Extract response text
	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no response from gemini")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Clean up response
	responseText = strings.TrimSpace(responseText)

	return responseText, nil
}

// GenerateResponseWithName sends a message to Gemini using a dynamic assistant name
func (c *GeminiClient) GenerateResponseWithName(ctx context.Context, assistantName string, message string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("gemini API key not configured")
	}

	if strings.TrimSpace(assistantName) == "" {
		assistantName = "Asisten"
	}

	systemPrompt := fmt.Sprintf(`Kamu adalah %s, asisten pribadi yang cerdas, membantu, dan ramah. 
Kamu dibuat untuk membantu pengguna dengan berbagai hal sehari-hari.
Selalu jawab dalam bahasa Indonesia yang sopan dan mudah dipahami.
Jika ditanya tentang identitasmu, katakan bahwa kamu adalah %s, asisten pribadi yang dibuat untuk membantu.
Jangan sebutkan bahwa kamu adalah AI atau bot kecuali ditanya secara spesifik.

Pesan pengguna: `, assistantName, assistantName)

	fullPrompt := systemPrompt + message

	requestData := GeminiRequest{
		Contents: []GeminiContent{{Parts: []GeminiPart{{Text: fullPrompt}}}},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("%s?key=%s", c.BaseURL, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	responseText := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	return responseText, nil
}

// Global Gemini client instance
var geminiClient *GeminiClient

// InitGemini initializes the global Gemini client
func InitGemini() {
	geminiClient = NewGeminiClient()
}

// GetGeminiResponse is a convenience function to get response from Gemini
func GetGeminiResponse(ctx context.Context, message string) (string, error) {
	if geminiClient == nil {
		InitGemini()
	}
	return geminiClient.GenerateResponse(ctx, message)
}

// GetGeminiResponseWithName is a convenience function to get response using a dynamic assistant name
func GetGeminiResponseWithName(ctx context.Context, assistantName string, message string) (string, error) {
	if geminiClient == nil {
		InitGemini()
	}
	return geminiClient.GenerateResponseWithName(ctx, assistantName, message)
}

// GetGeminiResponseWithMemory injects brief history and persists new turns
func GetGeminiResponseWithMemory(ctx context.Context, chatJID string, assistantName string, userMessage string) (string, error) {
	if geminiClient == nil {
		InitGemini()
	}

	var historyText string
	if MemStore != nil {
		history := MemStore.GetHistory(chatJID, assistantName, 6)
		for _, m := range history {
			if m.Role == "user" {
				historyText += "Pengguna: " + m.Text + "\n"
			} else if m.Role == "assistant" {
				historyText += assistantName + ": " + m.Text + "\n"
			}
		}
	}

	combined := userMessage
	if strings.TrimSpace(historyText) != "" {
		combined = "Riwayat percakapan singkat (konteks):\n" + historyText + "\nPertanyaan baru pengguna: " + userMessage
	}

	reply, err := geminiClient.GenerateResponseWithName(ctx, assistantName, combined)
	if err != nil {
		return "", err
	}

	if MemStore != nil {
		MemStore.AppendAndSave(chatJID, assistantName, "user", userMessage)
		MemStore.AppendAndSave(chatJID, assistantName, "assistant", reply)
	}

	return reply, nil
}
