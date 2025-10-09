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

// Image generation structures for Gemini 2.5 Flash Image
type GeminiImageRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiImageResponse struct {
	Candidates []GeminiImageCandidate `json:"candidates"`
}

type GeminiImageCandidate struct {
	Content GeminiImageContent `json:"content"`
}

type GeminiImageContent struct {
	Parts []GeminiImagePart `json:"parts"`
}

type GeminiImagePart struct {
	Text           string                `json:"text,omitempty"`
	InlineData     *GeminiInlineData     `json:"inlineData,omitempty"`
	ExecutableCode *GeminiExecutableCode `json:"executableCode,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiExecutableCode struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// GeminiClient holds the configuration for Gemini API
type GeminiClient struct {
	APIKey       string
	BaseURL      string
	ImageBaseURL string
	HTTPClient   *http.Client
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient() *GeminiClient {
	apiKey := os.Getenv("API_KEY_GEMINI")
	if apiKey == "" {
		log.Println("warning: API_KEY_GEMINI environment variable not set")
	}

	return &GeminiClient{
		APIKey:       apiKey,
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		ImageBaseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-preview-image-generation:generateContent",
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second, // Increased timeout for image generation
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

// GenerateImage sends a prompt to Gemini 2.5 Flash Image model and returns base64 encoded image
func (c *GeminiClient) GenerateImage(ctx context.Context, prompt string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("gemini API key not configured")
	}

	// Create image generation prompt
	imagePrompt := fmt.Sprintf("Generate an image based on this description: %s", prompt)

	// Prepare request payload with generation config for image generation
	requestData := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": imagePrompt,
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal image request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s?key=%s", c.ImageBaseURL, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create image request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	log.Printf("Sending image generation request to Gemini API...")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send image request: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Gemini API response status: %d", resp.StatusCode)

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Handle rate limiting specifically
		if resp.StatusCode == 429 {
			return "", fmt.Errorf("quota gemini habis atau rate limit tercapai. Silakan coba lagi nanti (status: %d)", resp.StatusCode)
		}
		return "", fmt.Errorf("gemini image API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parse response as generic map to handle flexible structure
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse image response: %v", err)
	}

	// Extract candidates
	candidates, ok := response["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid candidate format")
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no content in candidate")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no parts in content")
	}

	// Look for image data in parts
	for _, partInterface := range parts {
		part, ok := partInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for inline data (image)
		if inlineData, exists := part["inlineData"]; exists {
			inlineDataMap, ok := inlineData.(map[string]interface{})
			if !ok {
				continue
			}

			mimeType, _ := inlineDataMap["mimeType"].(string)
			data, _ := inlineDataMap["data"].(string)

			if mimeType != "" && data != "" {
				log.Printf("Found image data with mimeType: %s", mimeType)
				return data, nil
			}
		}
	}

	return "", fmt.Errorf("no image data found in response")
}

// GetGeminiImage is a convenience function to generate image from Gemini
func GetGeminiImage(ctx context.Context, prompt string) (string, error) {
	if geminiClient == nil {
		InitGemini()
	}
	return geminiClient.GenerateImage(ctx, prompt)
}
