package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"go.mau.fi/whatsmeow/types/events"
)

// Setup routes for the application
func SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Health check endpoint
	r.HandleFunc("/health", handleHealthCheck).Methods("GET")

	// Main status endpoint
	r.HandleFunc("/", handleMainStatus).Methods("GET")

	// Message sending endpoints
	r.HandleFunc("/send-message", handleSendMessage).Methods("POST")
	r.HandleFunc("/send-bulk-same-message", handleBulkSendSameMessage).Methods("POST")
	r.HandleFunc("/send-bulk-different-messages", handleBulkSendDifferentMessages).Methods("POST")

	// GitHub webhook endpoint
	r.HandleFunc("/github-webhook", handleGitHubWebhook).Methods("POST")

	// Groups endpoint
	r.HandleFunc("/groups", handleGetGroups).Methods("GET")

	// IDX market data endpoint
	r.HandleFunc("/idx", handleIDXData).Methods("GET")

	return r
}

// Handle health check
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"whatsapp":  WaClient.IsConnected(),
		"version":   "2.0.0",
	})
}

// Handle main status endpoint
func handleMainStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "WhatsApp Bot API is running",
		"connected": WaClient.IsConnected(),
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
}

// Handle get groups API endpoint
func handleGetGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !WaClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	groups, err := WaClient.GetJoinedGroups(context.Background())
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

// Handle IDX market data endpoint
func handleIDXData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("🔍 Fetching IDX market data...")

	data, err := GetIDXMarketData()
	if err != nil {
		log.Printf("❌ Error fetching IDX data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to fetch IDX data: " + err.Error(),
		})
		return
	}

	response := FormatIDXResponse(data)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      data,
		"formatted": response,
	})
}

// Event handler for WhatsApp events
func EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Optional: Skip messages from self to avoid loops
		// Uncomment the lines below if you want to prevent bot from responding to its own messages
		// if v.Info.IsFromMe {
		// 	return
		// }

		// Tampilkan informasi pesan yang masuk
		// if v.Info.IsGroup {
		// 	log.Printf("📱 GROUP MESSAGE - JID: %s", v.Info.Chat.String())
		// 	log.Printf("📱 GROUP NAME: %s", v.Info.Chat.User)
		// 	log.Printf("📱 FROM USER: %s", v.Info.Sender.String())
		// 	log.Printf("📱 MESSAGE: %s", v.Message.GetConversation())
		// 	log.Printf("📱 COPY THIS JID: %s", v.Info.Chat.String())
		// 	log.Println("=" + strings.Repeat("=", 50))
		// } else {
		// 	log.Printf("💬 INDIVIDUAL MESSAGE - From: %s", v.Info.Sender.String())
		// 	log.Printf("💬 MESSAGE: %s", v.Message.GetConversation())
		// }

		// Handle commands (case insensitive)
		message := getMessageText(v.Message)
		if strings.TrimSpace(message) == "" {
			return
		}
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
		} else if hasCommandPrefix(message, "/fiq") || hasCommandPrefix(message, "!fiq") {
			handleFiqCommand(v, message)
		} else if hasCommandPrefix(message, "/apik") || hasCommandPrefix(message, "!apik") {
			handleApikCommand(v, message)
		} else if hasCommandPrefix(message, "/idx") || hasCommandPrefix(message, "!idx") {
			handleIDXCommand(v)
		} else if hasCommandPrefix(message, "/img") || hasCommandPrefix(message, "!img") {
			handleImgCommand(v, message)
		}
	default:
		// Untuk event lain, tampilkan seperti biasa
		log.Printf("Event type: %T", evt)
	}
}

// Setup CORS middleware
func SetupCORS(r *mux.Router) http.Handler {
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	}).Handler(r)

	return handler
}
