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

	"whatsmeow-api/services/idx"
	"whatsmeow-api/utils"
	"whatsmeow-api/whatsapp"
)

func SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/health", handleHealthCheck).Methods("GET")

	r.HandleFunc("/", handleMainStatus).Methods("GET")

	r.HandleFunc("/send-message", handleSendMessage).Methods("POST")
	r.HandleFunc("/send-bulk-same-message", handleBulkSendSameMessage).Methods("POST")
	r.HandleFunc("/send-bulk-different-messages", handleBulkSendDifferentMessages).Methods("POST")

	r.HandleFunc("/github-webhook", handleGitHubWebhook).Methods("POST")

	r.HandleFunc("/viseron-webhook", handleViseronWebhook).Methods("POST")

	r.HandleFunc("/viseron-debug", handleViseronDebug).Methods("GET")

	r.HandleFunc("/groups", handleGetGroups).Methods("GET")

	r.HandleFunc("/idx", handleIDXData).Methods("GET")

	return r
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"whatsapp":  whatsapp.Client.IsConnected(),
		"version":   "2.0.0",
	})
}

func handleMainStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "WhatsApp Bot API is running",
		"connected": whatsapp.Client.IsConnected(),
		"timestamp": time.Now().Format(time.RFC3339),
		"endpoints": []string{
			"/health",
			"/send-message",
			"/send-bulk-same-message",
			"/send-bulk-different-messages",
			"/github-webhook (supports ?jid=<target_jid> parameter)",
			"/viseron-webhook",
			"/groups",
		},
	})
}

func handleGetGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !whatsapp.Client.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	groups, err := whatsapp.Client.GetJoinedGroups(context.Background())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

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

func handleIDXData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("[IDX] Fetching IDX market data for today...")

	data, err := idx.GetIDXMarketData(time.Time{})
	if err != nil {
		log.Printf("[Error] Error fetching IDX data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to fetch IDX data: " + err.Error(),
		})
		return
	}

	response := idx.FormatIDXResponse(data)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      data,
		"formatted": response,
	})
}

func EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:

		if v.Info.IsGroup {
			if utils.ShouldIgnoreGroup(v.Info.Chat.String()) {
				log.Printf("[Warning] Ignoring command from ignored group: %s", v.Info.Chat.String())
				return
			}
		}

		message := utils.GetMessageText(v.Message)
		if strings.TrimSpace(message) == "" {
			return
		}
		if utils.HasCommandPrefix(message, "/help") || utils.HasCommandPrefix(message, "!help") {
			handleHelpCommand(v)
		} else if utils.HasCommandPrefix(message, "/hallo") || utils.HasCommandPrefix(message, "!hallo") {
			handleHalloCommand(v)
		} else if utils.HasCommandPrefix(message, "/ping") || utils.HasCommandPrefix(message, "!ping") {
			handlePingCommand(v)
		} else if utils.HasCommandPrefix(message, "/status") || utils.HasCommandPrefix(message, "!status") {
			handleStatusCommand(v)
		} else if utils.HasCommandPrefix(message, "/info") || utils.HasCommandPrefix(message, "!info") {
			handleInfoCommand(v)
		} else if utils.HasCommandPrefix(message, "/groups") || utils.HasCommandPrefix(message, "!groups") {
			handleGroupsCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/test") || utils.HasCommandPrefix(message, "!test") {
			handleTestCommand(v)
		} else if utils.HasCommandPrefix(message, "/echo") || utils.HasCommandPrefix(message, "!echo") {
			handleEchoCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/fiq") || utils.HasCommandPrefix(message, "!fiq") {
			handleFiqCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/apik") || utils.HasCommandPrefix(message, "!apik") {
			handleApikCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/idx") || utils.HasCommandPrefix(message, "!idx") {
			handleIDXCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/img") || utils.HasCommandPrefix(message, "!img") {
			handleImgCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/cctv") || utils.HasCommandPrefix(message, "!cctv") {
			handleCCTVCommand(v, message)
		} else if utils.HasCommandPrefix(message, "/jid") || utils.HasCommandPrefix(message, "!jid") {
			handleJIDCommand(v, message)
		}
	default:

		log.Printf("Event type: %T", evt)
	}
}

func SetupCORS(r *mux.Router) http.Handler {
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	}).Handler(r)

	return handler
}
