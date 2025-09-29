package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

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

// Handle GitHub webhook
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
	if !WaClient.IsConnected() {
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

		err := sendMessageWithRetry(context.Background(), targetJID, message, 2)

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
