package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/cors"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type sendRequest struct {
	Secret  string `json:"secret"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

type bulkMessageRequest struct {
	Secret  string   `json:"secret"`
	Targets []string `json:"targets"`
	Message string   `json:"message"`
}

type bulkDifferentMessageRequest struct {
	Secret   string `json:"secret"`
	Messages []struct {
		Targets string `json:"targets"`
		Message string `json:"message"`
	} `json:"messages"`
}

// GitHub webhook payload structures
type GitHubWebhookPayload struct {
	Action      string       `json:"action,omitempty"`
	Repository  Repository   `json:"repository"`
	Sender      User         `json:"sender"`
	Pusher      User         `json:"pusher,omitempty"`
	Commits     []Commit     `json:"commits,omitempty"`
	HeadCommit  *Commit      `json:"head_commit,omitempty"`
	Ref         string       `json:"ref,omitempty"`
	Issue       *Issue       `json:"issue,omitempty"`
	PullRequest *PullRequest `json:"pull_request,omitempty"`
}

type Repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type User struct {
	Login   string `json:"login"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	HTMLURL string `json:"html_url"`
}

type Commit struct {
	ID        string   `json:"id"`
	Message   string   `json:"message"`
	URL       string   `json:"url"`
	Timestamp string   `json:"timestamp"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Author    struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"author"`
	Committer struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"committer"`
}

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
}

type PullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Merged  bool   `json:"merged"`
}

var waClient *whatsmeow.Client

// Helper function to check if message starts with command (case insensitive)
func hasCommandPrefix(message, command string) bool {
	messageLower := strings.ToLower(message)
	return strings.HasPrefix(messageLower, strings.ToLower(command))
}

// Helper function to check if message contains command (case insensitive)
func containsCommand(message, command string) bool {
	messageLower := strings.ToLower(message)
	return strings.Contains(messageLower, strings.ToLower(command))
}

// Helper function to get target phone numbers and group JIDs from environment
func getNotificationTargets() []string {
	targets := os.Getenv("NOTIFICATION_TARGETS")
	if targets == "" {
		return []string{}
	}
	return strings.Split(targets, ",")
}

// Helper function to determine if a target is a group JID or phone number
func isGroupJID(target string) bool {
	// WhatsApp group JIDs end with @g.us
	return strings.HasSuffix(target, "@g.us")
}

// Helper function to create appropriate JID for target
func createTargetJID(target string) types.JID {
	target = strings.TrimSpace(target)

	if isGroupJID(target) {
		// It's already a group JID, parse it directly
		jid, err := types.ParseJID(target)
		if err != nil {
			log.Printf("Invalid group JID format: %s, error: %v", target, err)
			// Return empty JID if parsing fails
			return types.JID{}
		}
		return jid
	} else {
		// It's a phone number, normalize and create individual JID
		normalizedTarget := normalizePhoneNumber(target)
		return types.NewJID(normalizedTarget, types.DefaultUserServer)
	}
}

// Helper function to get pusher name with fallback
func getPusherName(payload *GitHubWebhookPayload) string {
	if payload.Pusher.Name != "" {
		return payload.Pusher.Name
	}
	if payload.Pusher.Login != "" {
		return payload.Pusher.Login
	}
	if payload.Sender.Login != "" {
		return payload.Sender.Login
	}
	return "Unknown"
}

// Helper function to get file changes summary
func getFileChangesSummary(commit Commit) string {
	var changes []string

	if len(commit.Added) > 0 {
		changes = append(changes, fmt.Sprintf("➕ %d added", len(commit.Added)))
	}
	if len(commit.Modified) > 0 {
		changes = append(changes, fmt.Sprintf("📝 %d modified", len(commit.Modified)))
	}
	if len(commit.Removed) > 0 {
		changes = append(changes, fmt.Sprintf("➖ %d removed", len(commit.Removed)))
	}

	if len(changes) == 0 {
		return ""
	}

	return " (" + strings.Join(changes, ", ") + ")"
}

// Format GitHub event messages
func formatGitHubMessage(eventType string, payload *GitHubWebhookPayload) string {
	repo := payload.Repository.FullName

	switch eventType {
	case "push":
		if len(payload.Commits) == 0 {
			pusherName := getPusherName(payload)
			return fmt.Sprintf("🔄 *Push Event*\n📁 *Repository:* %s\n👤 *Pusher:* %s\n🌿 *Branch:* %s\n\n_No commits in this push_",
				repo, pusherName, strings.TrimPrefix(payload.Ref, "refs/heads/"))
		}

		commitCount := len(payload.Commits)
		branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		pusherName := getPusherName(payload)

		message := fmt.Sprintf("🔄 *Push Event*\n📁 *Repository:* %s\n👤 *Pusher:* %s\n🌿 *Branch:* %s\n📝 *Commits:* %d\n\n",
			repo, pusherName, branch, commitCount)

		// Show up to 3 commits with enhanced details
		for i, commit := range payload.Commits {
			if i >= 3 {
				message += fmt.Sprintf("_... and %d more commits_\n", commitCount-3)
				break
			}
			shortID := commit.ID[:7]

			// Add file changes summary
			fileChanges := getFileChangesSummary(commit)

			// Format commit message (truncate if too long)
			commitMsg := commit.Message
			if len(commitMsg) > 80 {
				commitMsg = commitMsg[:77] + "..."
			}

			message += fmt.Sprintf("🔹 `%s` %s%s\n", shortID, commitMsg, fileChanges)
		}

		// Add repository link
		message += fmt.Sprintf("\n🔗 *View Repository:* %s", payload.Repository.HTMLURL)

		return message

	case "issues":
		action := payload.Action
		issue := payload.Issue
		actionEmoji := "🐛"

		switch action {
		case "opened":
			actionEmoji = "🆕"
		case "closed":
			actionEmoji = "✅"
		case "reopened":
			actionEmoji = "🔄"
		}

		message := fmt.Sprintf("%s *Issue %s*\n📁 *Repository:* %s\n👤 *User:* %s\n📋 *Issue #%d:* %s\n🔗 *Link:* %s",
			actionEmoji, strings.Title(action), repo, payload.Sender.Login, issue.Number, issue.Title, issue.HTMLURL)
		return message

	case "pull_request":
		action := payload.Action
		pr := payload.PullRequest
		actionEmoji := "🔀"

		switch action {
		case "opened":
			actionEmoji = "🆕"
		case "closed":
			if pr.Merged {
				actionEmoji = "✅"
				action = "merged"
			} else {
				actionEmoji = "❌"
			}
		case "reopened":
			actionEmoji = "🔄"
		}

		message := fmt.Sprintf("%s *Pull Request %s*\n📁 *Repository:* %s\n👤 *User:* %s\n📋 *PR #%d:* %s\n🔗 *Link:* %s",
			actionEmoji, strings.Title(action), repo, payload.Sender.Login, pr.Number, pr.Title, pr.HTMLURL)
		return message

	case "release":
		message := fmt.Sprintf("🚀 *Release %s*\n📁 *Repository:* %s\n👤 *User:* %s\n🔗 *Link:* %s",
			strings.Title(payload.Action), repo, payload.Sender.Login, payload.Repository.HTMLURL)
		return message

	default:
		return fmt.Sprintf("📢 *GitHub Event: %s*\n📁 *Repository:* %s\n👤 *User:* %s\n🔗 *Link:* %s",
			strings.Title(eventType), repo, payload.Sender.Login, payload.Repository.HTMLURL)
	}
}

func normalizePhoneNumber(phone string) string {
	re := regexp.MustCompile(`\D`)
	phone = re.ReplaceAllString(phone, "")

	if strings.HasPrefix(phone, "08") {
		phone = "628" + phone[2:]
	}

	if strings.HasPrefix(phone, "8") && !strings.HasPrefix(phone, "62") {
		phone = "62" + phone
	}

	if strings.HasPrefix(phone, "+62") {
		phone = phone[1:]
	}

	if !strings.HasPrefix(phone, "62") {
		phone = "62" + phone
	}

	return phone
}

func sendMessageWithRetry(targetJID types.JID, message string, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		_, err = waClient.SendMessage(context.Background(), targetJID, &waE2E.Message{
			Conversation: proto.String(message),
		})

		if err == nil {
			return nil
		}

		log.Printf("Attempt %d failed for %s: %v", i+1, targetJID, err)

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return err
}

func handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	// Add detailed logging for debugging
	log.Printf("🔔 GitHub webhook received: %s %s", r.Method, r.URL.Path)
	log.Printf("🔔 Headers: %v", r.Header)

	w.Header().Set("Content-Type", "application/json")

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read request body"})
		return
	}

	log.Printf("🔔 Request body length: %d bytes", len(body))

	// Skip signature verification since no secret is configured
	log.Printf("🔔 Webhook signature verification: disabled")

	// Get event type from header
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		log.Printf("❌ Missing X-GitHub-Event header")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing X-GitHub-Event header"})
		return
	}

	log.Printf("🔔 GitHub event type: %s", eventType)

	// Parse the webhook payload
	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("❌ Failed to parse JSON payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse JSON payload"})
		return
	}

	log.Printf("🔔 Repository: %s", payload.Repository.FullName)

	// Check if WhatsApp client is connected
	if !waClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	// Get notification targets
	var targets []string

	// Check if custom JID is provided in query parameter
	customJID := r.URL.Query().Get("jid")
	if customJID != "" {
		// Use custom JID from query parameter
		targets = []string{customJID}
		log.Printf("🎯 Using custom JID from query parameter: %s", customJID)
	} else {
		// Use default targets from environment
		targets = getNotificationTargets()
		if len(targets) == 0 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "Webhook received but no notification targets configured",
				"event":  eventType,
			})
			return
		}
		log.Printf("🎯 Using default targets from environment: %d targets", len(targets))
	}

	// Format the message based on event type
	message := formatGitHubMessage(eventType, &payload)

	// Send notifications to all targets
	results := make([]map[string]interface{}, len(targets))
	successCount := 0

	for i, target := range targets {
		targetJID := createTargetJID(target)

		// Skip if JID creation failed
		if targetJID.IsEmpty() {
			results[i] = map[string]interface{}{
				"target":  target,
				"success": false,
				"error":   "Invalid JID format",
			}
			log.Printf("Skipping invalid target: %s", target)
			continue
		}

		targetType := "individual"
		displayTarget := target
		if isGroupJID(target) {
			targetType = "group"
		} else {
			displayTarget = normalizePhoneNumber(strings.TrimSpace(target))
		}

		log.Printf("Sending GitHub notification (%s) to %s: %s", eventType, targetType, displayTarget)

		err := sendMessageWithRetry(targetJID, message, 2)

		results[i] = map[string]interface{}{
			"target":      displayTarget,
			"target_type": targetType,
			"success":     err == nil,
		}

		if err != nil {
			results[i]["error"] = err.Error()
			log.Printf("Failed to send GitHub notification to %s %s: %v", targetType, displayTarget, err)
		} else {
			successCount++
		}

		// Small delay between messages
		if i < len(targets)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "Webhook processed",
		"event":         eventType,
		"repository":    payload.Repository.FullName,
		"targets_sent":  successCount,
		"total_targets": len(targets),
		"custom_jid":    customJID != "",
		"target_source": func() string {
			if customJID != "" {
				return "query_parameter"
			}
			return "environment"
		}(),
		"results": results,
	})
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	SECRET := os.Getenv("API_SECRET")
	if SECRET == "" {
		SECRET = "default-secret"
	}

	if req.Secret != SECRET {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if !waClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	targetJID := createTargetJID(req.Target)

	// Check if JID creation failed
	if targetJID.IsEmpty() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "Invalid target format (must be phone number or group JID)",
			"target": req.Target,
		})
		return
	}

	targetType := "individual"
	displayTarget := req.Target
	if isGroupJID(req.Target) {
		targetType = "group"
	} else {
		displayTarget = normalizePhoneNumber(req.Target)
	}

	log.Printf("Sending message to %s: %s (original: %s)", targetType, displayTarget, req.Target)

	err := sendMessageWithRetry(targetJID, req.Message, 3)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":           err.Error(),
			"original_target": req.Target,
			"target_type":     targetType,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "Success",
		"target":      displayTarget,
		"target_type": targetType,
	})
}

func handleBulkSendSameMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req bulkMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	SECRET := os.Getenv("API_SECRET")
	if SECRET == "" {
		SECRET = "default-secret"
	}

	if req.Secret != SECRET {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if !waClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	results := make([]map[string]interface{}, len(req.Targets))

	for i, target := range req.Targets {
		targetJID := createTargetJID(target)

		// Skip if JID creation failed
		if targetJID.IsEmpty() {
			results[i] = map[string]interface{}{
				"original_target": target,
				"success":         false,
				"error":           "Invalid JID format",
			}
			log.Printf("Skipping invalid bulk target: %s", target)
			continue
		}

		targetType := "individual"
		displayTarget := target
		if isGroupJID(target) {
			targetType = "group"
		} else {
			displayTarget = normalizePhoneNumber(target)
		}

		log.Printf("Sending bulk message %d/%d to %s: %s", i+1, len(req.Targets), targetType, displayTarget)

		err := sendMessageWithRetry(targetJID, req.Message, 2)

		results[i] = map[string]interface{}{
			"original_target": target,
			"target":          displayTarget,
			"target_type":     targetType,
			"success":         err == nil,
		}

		if err != nil {
			results[i]["error"] = err.Error()
			log.Printf("Failed to send bulk message to %s %s: %v", targetType, displayTarget, err)
		}

		if i < len(req.Targets)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "Bulk same message processing completed",
		"results": results,
	})
}

func handleBulkSendDifferentMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req bulkDifferentMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	SECRET := os.Getenv("API_SECRET")
	if SECRET == "" {
		SECRET = "default-secret"
	}

	if req.Secret != SECRET {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if !waClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	results := make([]map[string]interface{}, len(req.Messages))

	for i, msg := range req.Messages {
		targetJID := createTargetJID(msg.Targets)

		// Skip if JID creation failed
		if targetJID.IsEmpty() {
			results[i] = map[string]interface{}{
				"original_target": msg.Targets,
				"success":         false,
				"error":           "Invalid JID format",
				"message":         msg.Message,
			}
			log.Printf("Skipping invalid different message target: %s", msg.Targets)
			continue
		}

		targetType := "individual"
		displayTarget := msg.Targets
		if isGroupJID(msg.Targets) {
			targetType = "group"
		} else {
			displayTarget = normalizePhoneNumber(msg.Targets)
		}

		log.Printf("Sending different message %d/%d to %s: %s", i+1, len(req.Messages), targetType, displayTarget)

		err := sendMessageWithRetry(targetJID, msg.Message, 2)

		results[i] = map[string]interface{}{
			"original_target": msg.Targets,
			"target":          displayTarget,
			"target_type":     targetType,
			"success":         err == nil,
			"message":         msg.Message,
		}

		if err != nil {
			results[i]["error"] = err.Error()
			log.Printf("Failed to send different message to %s %s: %v", targetType, displayTarget, err)
		}

		if i < len(req.Messages)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "Bulk different messages processing completed",
		"results": results,
	})
}

// Handle help command from WhatsApp message
func handleHelpCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	helpMessage := `🤖 *WhatsApp Bot - Bantuan Penggunaan*

*📋 Daftar Perintah:*

*!help* atau */help*
Menampilkan bantuan dan cara penggunaan bot

*!hallo* atau */hallo*
Menyapa bot dengan ramah

*!groups* atau */groups*
Menampilkan daftar grup yang diikuti bot

*!ping* atau */ping*
Cek apakah bot sedang aktif

*!status* atau */status*
Menampilkan status koneksi bot

*!info* atau */info*
Menampilkan informasi tentang bot

*!test* atau */test*
Test apakah bot berfungsi dengan baik

*!echo [teks]* atau */echo [teks]*
Mengulang pesan yang dikirim

*💡 Tips:*
- Semua perintah bisa menggunakan ! atau /
- Bot akan merespons secara otomatis
- Gunakan perintah di chat pribadi atau grup

*📞 Dukungan:*
Jika ada pertanyaan, silakan hubungi administrator bot.`

	// Send response
	err := sendMessageWithRetry(v.Info.Chat, helpMessage, 2)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

// Handle hallo command from WhatsApp message
func handleHalloCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	senderName := "teman"
	if v.Info.PushName != "" {
		senderName = v.Info.PushName
	}

	halloMessage := fmt.Sprintf("👋 Hallo %s! 😊\n\nSenang bertemu denganmu! Ada yang bisa saya bantu hari ini?\n\nKetik *!help* untuk melihat semua perintah yang tersedia.", senderName)

	err := sendMessageWithRetry(v.Info.Chat, halloMessage, 2)
	if err != nil {
		log.Printf("Failed to send hallo message: %v", err)
	}
}

// Handle ping command from WhatsApp message
func handlePingCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	pingMessage := "🏓 Pong! Bot sedang aktif dan siap melayani. ⚡"

	err := sendMessageWithRetry(v.Info.Chat, pingMessage, 2)
	if err != nil {
		log.Printf("Failed to send ping message: %v", err)
	}
}

// Handle status command from WhatsApp message
func handleStatusCommand(v *events.Message) {
	if !waClient.IsConnected() {
		sendMessageWithRetry(v.Info.Chat, "❌ Bot sedang tidak terhubung ke WhatsApp", 2)
		return
	}

	statusMessage := fmt.Sprintf(`📊 *Status Bot*

✅ *Koneksi WhatsApp:* Terhubung
🤖 *Bot Status:* Aktif
⏰ *Waktu:* %s
🔄 *Uptime:* Bot sedang berjalan

Semua sistem berfungsi dengan baik!`, time.Now().Format("02 Jan 2006, 15:04:05 WIB"))

	err := sendMessageWithRetry(v.Info.Chat, statusMessage, 2)
	if err != nil {
		log.Printf("Failed to send status message: %v", err)
	}
}

// Handle info command from WhatsApp message
func handleInfoCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	infoMessage := `ℹ️ *Informasi Bot*

🤖 *Nama:* WhatsApp Bot API
📝 *Versi:* 2.0.0
👨‍💻 *Developer:* WhatsApp Bot Team
🌐 *Bahasa:* Go (Golang)
📱 *Platform:* WhatsApp Web
⚙️ *Fitur:* Auto-reply, Group Management, Message API

Bot ini dibuat untuk memudahkan komunikasi dan otomasi pesan WhatsApp melalui API.`

	err := sendMessageWithRetry(v.Info.Chat, infoMessage, 2)
	if err != nil {
		log.Printf("Failed to send info message: %v", err)
	}
}

// Handle test command from WhatsApp message
func handleTestCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	testMessage := `🧪 *Test Bot Response*

✅ *Bot Status:* Aktif dan berfungsi dengan baik
✅ *Connection:* WhatsApp terhubung
✅ *Commands:* Case insensitive aktif
✅ *Web Support:* WhatsApp Web didukung

*Test berhasil!* Bot siap menerima perintah dalam berbagai format:
• huruf BESAR: !HELP, !PING, !STATUS
• huruf kecil: !help, !ping, !status
• Campuran: !HeLp, !PiNg, !StAtUs

Semua format akan dikenali dengan benar! 🎉`

	err := sendMessageWithRetry(v.Info.Chat, testMessage, 2)
	if err != nil {
		log.Printf("Failed to send test message: %v", err)
	}
}

// Handle echo command from WhatsApp message
func handleEchoCommand(v *events.Message, originalMessage string) {
	if !waClient.IsConnected() {
		return
	}

	// Remove the command prefix and get the text to echo
	var echoText string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!echo ") {
		echoText = strings.TrimSpace(originalMessage[6:]) // Remove "!echo "
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/echo ") {
		echoText = strings.TrimSpace(originalMessage[6:]) // Remove "/echo "
	} else {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	if echoText == "" {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	echoResponse := fmt.Sprintf("🔊 *Echo Response:*\n\n%s", echoText)

	err := sendMessageWithRetry(v.Info.Chat, echoResponse, 2)
	if err != nil {
		log.Printf("Failed to send echo message: %v", err)
	}
}

// Handle groups command from WhatsApp message
func handleGroupsCommand(v *events.Message) {
	if !waClient.IsConnected() {
		return
	}

	// Get all groups
	groups, err := waClient.GetJoinedGroups()
	if err != nil {
		log.Printf("Failed to get joined groups: %v", err)
		sendMessageWithRetry(v.Info.Chat, "❌ Gagal mengambil daftar grup: "+err.Error(), 2)
		return
	}

	if len(groups) == 0 {
		sendMessageWithRetry(v.Info.Chat, "📝 Tidak ada grup yang diikuti.", 2)
		return
	}

	// Format groups list
	message := fmt.Sprintf("📋 *Daftar Grup yang Diikuti* (%d grup)\n\n", len(groups))

	for i, group := range groups {
		if i >= 20 { // Limit to 20 groups to avoid message being too long
			message += fmt.Sprintf("_... dan %d grup lainnya_\n", len(groups)-20)
			break
		}

		groupName := group.Name
		if groupName == "" {
			groupName = "Tanpa Nama"
		}

		message += fmt.Sprintf("🏷️ *%s*\n", groupName)
		message += fmt.Sprintf("🆔 `%s`\n", group.JID.String())
	}

	message += "💡 _Gunakan /groups untuk melihat daftar ini lagi_"

	// Send response
	err = sendMessageWithRetry(v.Info.Chat, message, 2)
	if err != nil {
		log.Printf("Failed to send groups list: %v", err)
	}
}

// API endpoint to get groups
func handleGetGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !waClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	groups, err := waClient.GetJoinedGroups()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Format response
	groupList := make([]map[string]interface{}, len(groups))
	for i, group := range groups {
		groupList[i] = map[string]interface{}{
			"jid":        group.JID.String(),
			"name":       group.Name,
			"owner":      group.OwnerJID.String(),
			"created_at": group.GroupCreated.Unix(),
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "Success",
		"total":     len(groups),
		"groups":    groupList,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Optional: Skip messages from self to avoid loops
		// Uncomment the lines below if you want to prevent bot from responding to its own messages
		// if v.Info.IsFromMe {
		// 	return
		// }

		// Tampilkan informasi pesan yang masuk
		if v.Info.IsGroup {
			log.Printf("📱 GROUP MESSAGE - JID: %s", v.Info.Chat.String())
			log.Printf("📱 GROUP NAME: %s", v.Info.Chat.User)
			log.Printf("📱 FROM USER: %s", v.Info.Sender.String())
			log.Printf("📱 MESSAGE: %s", v.Message.GetConversation())
			log.Printf("📱 COPY THIS JID: %s", v.Info.Chat.String())
			log.Println("=" + strings.Repeat("=", 50))
		} else {
			log.Printf("💬 INDIVIDUAL MESSAGE - From: %s", v.Info.Sender.String())
			log.Printf("💬 MESSAGE: %s", v.Message.GetConversation())
		}

		// Handle commands (case insensitive)
		message := v.Message.GetConversation()
		if hasCommandPrefix(message, "/help") || hasCommandPrefix(message, "!help") {
			handleHelpCommand(v)
		} else if hasCommandPrefix(message, "/hallo") || hasCommandPrefix(message, "!hallo") {
			handleHalloCommand(v)
		} else if hasCommandPrefix(message, "/ping") || hasCommandPrefix(message, "!ping") {
			handlePingCommand(v)
		} else if hasCommandPrefix(message, "/status") || hasCommandPrefix(message, "!status") {
			handleStatusCommand(v)
		} else if hasCommandPrefix(message, "/info") || hasCommandPrefix(message, "!info") {
			handleInfoCommand(v)
		} else if hasCommandPrefix(message, "/groups") || hasCommandPrefix(message, "!groups") {
			handleGroupsCommand(v)
		} else if hasCommandPrefix(message, "/test") || hasCommandPrefix(message, "!test") {
			handleTestCommand(v)
		} else if hasCommandPrefix(message, "/echo") || hasCommandPrefix(message, "!echo") {
			handleEchoCommand(v, message)
		}
	default:
		// Untuk event lain, tampilkan seperti biasa
		log.Printf("Event type: %T", evt)
	}
}

func main() {
	ctx := context.Background()

	logger := waLog.Stdout("whatsapp", "INFO", true)

	container, err := sqlstore.New(ctx, "sqlite3", "file:store.db?_foreign_keys=on", logger)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Fatalf("Failed to get device: %v", err)
	}

	waClient = whatsmeow.NewClient(deviceStore, logger)
	waClient.AddEventHandler(eventHandler)

	if waClient.Store.ID == nil {
		qrChan, _ := waClient.GetQRChannel(ctx)
		err = waClient.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("QR Code:")
				fmt.Println(evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = waClient.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
	}

	r := mux.NewRouter()

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"whatsapp":  waClient.IsConnected(),
			"version":   "2.0.0",
		})
	}).Methods("GET")

	// Main status endpoint
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "WhatsApp Bot API is running",
			"connected": waClient.IsConnected(),
			"timestamp": time.Now().Format(time.RFC3339),
			"endpoints": []string{
				"/health",
				"/send-message",
				"/send-bulk-same-message",
				"/send-bulk-different-messages",
				"/github-webhook (supports ?jid=<target_jid> parameter)",
				"/groups",
			},
		})
	}).Methods("GET")

	r.HandleFunc("/send-message", handleSendMessage).Methods("POST")
	r.HandleFunc("/send-bulk-same-message", handleBulkSendSameMessage).Methods("POST")
	r.HandleFunc("/send-bulk-different-messages", handleBulkSendDifferentMessages).Methods("POST")
	r.HandleFunc("/github-webhook", handleGitHubWebhook).Methods("POST")
	r.HandleFunc("/groups", handleGetGroups).Methods("GET")

	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	}).Handler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("🚀 WhatsApp Bot Server starting...")
	log.Printf("🌐 Port: %s", port)
	log.Printf("🔗 WhatsApp Connected: %t", waClient.IsConnected())
	log.Printf("📋 Available endpoints:")
	log.Printf("   GET  / - Status")
	log.Printf("   GET  /health - Health check")
	log.Printf("   GET  /groups - Get joined groups")
	log.Printf("   POST /send-message - Send message")
	log.Printf("   POST /send-bulk-same-message - Bulk same message")
	log.Printf("   POST /send-bulk-different-messages - Bulk different messages")
	log.Printf("   POST /github-webhook - GitHub webhook (supports ?jid=<target_jid>)")
	log.Printf("✅ Server is ready and listening on port %s", port)

	log.Fatal(http.ListenAndServe(":"+port, handler))
}
