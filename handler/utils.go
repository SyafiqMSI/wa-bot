package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

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

// Normalize phone number function
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

// Send message with retry mechanism
func sendMessageWithRetry(ctx context.Context, targetJID types.JID, message string, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		_, err = WaClient.SendMessage(ctx, targetJID, &waE2E.Message{
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

// Extract human-readable text from various WhatsApp message types
func getMessageText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	// Plain text
	if txt := msg.GetConversation(); txt != "" {
		return txt
	}

	// Extended text
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if t := ext.GetText(); t != "" {
			return t
		}
	}

	// Media captions
	if im := msg.GetImageMessage(); im != nil {
		if cap := im.GetCaption(); cap != "" {
			return cap
		}
	}
	if vm := msg.GetVideoMessage(); vm != nil {
		if cap := vm.GetCaption(); cap != "" {
			return cap
		}
	}
	if dm := msg.GetDocumentMessage(); dm != nil {
		if cap := dm.GetCaption(); cap != "" {
			return cap
		}
	}

	// Button/list/template responses
	if br := msg.GetButtonsResponseMessage(); br != nil {
		if txt := br.GetSelectedDisplayText(); txt != "" {
			return txt
		}
		if id := br.GetSelectedButtonID(); id != "" {
			return id
		}
	}
	if lr := msg.GetListResponseMessage(); lr != nil {
		if single := lr.GetSingleSelectReply(); single != nil {
			// Prefer the selected row ID as the command token; display text may be empty
			if id := single.GetSelectedRowID(); id != "" {
				return id
			}
		}
		if title := lr.GetTitle(); title != "" {
			return title
		}
	}
	if tr := msg.GetTemplateButtonReplyMessage(); tr != nil {
		if txt := tr.GetSelectedDisplayText(); txt != "" {
			return txt
		}
		if id := tr.GetSelectedID(); id != "" {
			return id
		}
	}

	// Interactive responses (newer flows)
	if ir := msg.GetInteractiveResponseMessage(); ir != nil {
		if body := ir.GetBody(); body != nil {
			if t := body.GetText(); t != "" {
				return t
			}
		}
		if native := ir.GetNativeFlowResponseMessage(); native != nil {
			if p := native.GetParamsJSON(); p != "" {
				return p
			}
		}
	}

	// Wrapped messages
	if ep := msg.GetEphemeralMessage(); ep != nil {
		return getMessageText(ep.GetMessage())
	}
	if dv := msg.GetDeviceSentMessage(); dv != nil {
		return getMessageText(dv.GetMessage())
	}

	return ""
}
