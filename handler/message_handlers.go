package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// Handle send message
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

	if !WaClient.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	targetJID := createTargetJID(req.Target)

	// Check if JID creation failed
	if targetJID.IsEmpty() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
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

	err := sendMessageWithRetry(context.Background(), targetJID, req.Message, 3)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":           err.Error(),
			"original_target": req.Target,
			"target_type":     targetType,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "Success",
		"target":      displayTarget,
		"target_type": targetType,
	})
}

// Handle bulk send same message
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

	if !WaClient.IsConnected() {
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

		err := sendMessageWithRetry(context.Background(), targetJID, req.Message, 2)

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

// Handle bulk send different messages
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

	if !WaClient.IsConnected() {
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

		err := sendMessageWithRetry(context.Background(), targetJID, msg.Message, 2)

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
