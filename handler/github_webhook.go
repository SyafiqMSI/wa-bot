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

	"whatsmeow-api/domain"
	"whatsmeow-api/utils"
	"whatsmeow-api/whatsapp"
)

func formatGitHubMessage(eventType string, payload *domain.GitHubWebhookPayload) string {
	repo := payload.Repository.FullName

	switch eventType {
	case "push":
		if len(payload.Commits) == 0 {
			pusherName := utils.GetPusherName(payload)
			return fmt.Sprintf("[Push Event]\nRepository: %s\nPusher: %s\nBranch: %s\n\n_No commits in this push_",
				repo, pusherName, strings.TrimPrefix(payload.Ref, "refs/heads/"))
		}

		commitCount := len(payload.Commits)
		branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		pusherName := utils.GetPusherName(payload)

		message := fmt.Sprintf("[Push Event]\nRepository: %s\nPusher: %s\nBranch: %s\nCommits: %d\n\n",
			repo, pusherName, branch, commitCount)

		for i, commit := range payload.Commits {
			if i >= 3 {
				message += fmt.Sprintf("_... and %d more commits_\n", commitCount-3)
				break
			}
			shortID := commit.ID[:7]

			fileChanges := utils.GetFileChangesSummary(commit)

			commitMsg := commit.Message
			if len(commitMsg) > 80 {
				commitMsg = commitMsg[:77] + "..."
			}

			message += fmt.Sprintf("- `%s` %s%s\n", shortID, commitMsg, fileChanges)
		}

		message += fmt.Sprintf("\nView Repository: %s", payload.Repository.HTMLURL)

		return message

	case "issues":
		action := payload.Action
		issue := payload.Issue
		actionPrefix := "[Issue]"
		switch action {
		case "opened":
			actionPrefix = "[New Issue]"
		case "closed":
			actionPrefix = "[Closed Issue]"
		case "reopened":
			actionPrefix = "[Reopened Issue]"
		}

		message := fmt.Sprintf("%s\nRepository: %s\nUser: %s\nIssue #%d: %s\nLink: %s",
			actionPrefix, repo, payload.Sender.Login, issue.Number, issue.Title, issue.HTMLURL)
		return message

	case "pull_request":
		action := payload.Action
		pr := payload.PullRequest
		actionPrefix := "[Pull Request]"
		switch action {
		case "opened":
			actionPrefix = "[New PR]"
		case "closed":
			if pr.Merged {
				actionPrefix = "[Merged PR]"
			} else {
				actionPrefix = "[Closed PR]"
			}
		case "reopened":
			actionPrefix = "[Reopened PR]"
		}

		message := fmt.Sprintf("%s\nRepository: %s\nUser: %s\nPR #%d: %s\nLink: %s",
			actionPrefix, repo, payload.Sender.Login, pr.Number, pr.Title, pr.HTMLURL)
		return message

	case "release":
		message := fmt.Sprintf("[Release %s]\nRepository: %s\nUser: %s\nLink: %s",
			strings.Title(payload.Action), repo, payload.Sender.Login, payload.Repository.HTMLURL)
		return message

	default:
		return fmt.Sprintf("[GitHub Event: %s]\nRepository: %s\nUser: %s\nLink: %s",
			strings.Title(eventType), repo, payload.Sender.Login, payload.Repository.HTMLURL)
	}
}

func handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {

	log.Printf("[github] webhook received: %s %s", r.Method, r.URL.Path)
	log.Printf("[github] Headers: %v", r.Header)

	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[github] Failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read request body"})
		return
	}

	log.Printf("[github] Request body length: %d bytes", len(body))

	log.Printf("[github] Webhook signature verification: disabled")

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		log.Printf("[github] Missing X-GitHub-Event header")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing X-GitHub-Event header"})
		return
	}

	log.Printf("[github] event type: %s", eventType)

	var payload domain.GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[github] Failed to parse JSON payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse JSON payload"})
		return
	}

	log.Printf("[github] Repository: %s", payload.Repository.FullName)

	if !whatsapp.Client.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp client not connected"})
		return
	}

	var targets []string

	customJID := r.URL.Query().Get("jid")
	if customJID != "" {

		targets = []string{customJID}
		log.Printf("[github] Using custom JID from query parameter: %s", customJID)
	} else {

		targets = utils.GetNotificationTargets()
		if len(targets) == 0 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "Webhook received but no notification targets configured",
				"event":  eventType,
			})
			return
		}
		log.Printf("[github] Using default targets from environment: %d targets", len(targets))
	}

	message := formatGitHubMessage(eventType, &payload)

	results := make([]map[string]interface{}, len(targets))
	successCount := 0

	for i, target := range targets {
		targetJID := utils.CreateTargetJID(target)

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
		if utils.IsGroupJID(target) {
			targetType = "group"
		} else {
			displayTarget = utils.NormalizePhoneNumber(strings.TrimSpace(target))
		}

		log.Printf("Sending GitHub notification (%s) to %s: %s", eventType, targetType, displayTarget)

		err := utils.SendMessageWithRetry(context.Background(), targetJID, message, 2)

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
