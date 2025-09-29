package handler

import (
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// WhatsApp client instance
var WaClient *whatsmeow.Client

// Message request structures
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

// Event message type for WhatsApp events
type MessageEvent = events.Message
