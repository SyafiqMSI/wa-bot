package domain

import (
	"go.mau.fi/whatsmeow/types/events"
)

type SendRequest struct {
	Secret  string `json:"secret"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

type BulkMessageRequest struct {
	Secret  string   `json:"secret"`
	Targets []string `json:"targets"`
	Message string   `json:"message"`
}

type BulkDifferentMessageRequest struct {
	Secret   string `json:"secret"`
	Messages []struct {
		Targets string `json:"targets"`
		Message string `json:"message"`
	} `json:"messages"`
}

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

type MessageEvent = events.Message

type ViseronPayload struct {
	Camera         string          `json:"camera,omitempty"`
	CameraName     string          `json:"camera_name,omitempty"`
	EventType      string          `json:"event_type,omitempty"`
	TriggerTime    string          `json:"trigger_time,omitempty"`
	Objects        []ViseronObject `json:"objects,omitempty"`
	SnapshotURL    string          `json:"snapshot_url,omitempty"`
	ViseronBaseURL string          `json:"-"`
}

type ViseronObject struct {
	Label      string  `json:"label,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type IDXData struct {
	Date     string
	RUPS     []string
	UMA      []string
	Suspensi []string
	Dividend []DividendData
}

type DividendData struct {
	Code    string
	Amount  string
	Yield   string
	Price   string
	CumDate string
	ExDate  string
}
