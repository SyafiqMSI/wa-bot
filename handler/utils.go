package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
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

// Helper function to get list of group JIDs that should be ignored (no response)
func getNoResponseGroups() []string {
	noResponse := os.Getenv("NO_RESPONSE")
	if noResponse == "" {
		return []string{}
	}
	// Split by semicolon and trim spaces
	jids := strings.Split(noResponse, ";")
	result := make([]string, 0, len(jids))
	for _, jid := range jids {
		trimmed := strings.TrimSpace(jid)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Helper function to check if a group JID should be ignored
func shouldIgnoreGroup(chatJID string) bool {
	noResponseGroups := getNoResponseGroups()
	if len(noResponseGroups) == 0 {
		return false
	}
	
	// Normalize the chat JID for comparison
	chatJID = strings.TrimSpace(chatJID)
	
	for _, ignoredJID := range noResponseGroups {
		if strings.TrimSpace(ignoredJID) == chatJID {
			return true
		}
	}
	return false
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

// Send image message with base64 data
func sendImageWithRetry(ctx context.Context, targetJID types.JID, imageBase64 string, caption string, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		// Decode base64 image data
		imageData, decodeErr := base64.StdEncoding.DecodeString(imageBase64)
		if decodeErr != nil {
			return fmt.Errorf("failed to decode base64 image: %v", decodeErr)
		}

		log.Printf("Image data size: %d bytes", len(imageData))

		// Check if image is too large (WhatsApp limit is around 16MB)
		if len(imageData) > 15*1024*1024 {
			return fmt.Errorf("image too large: %d bytes (max 15MB)", len(imageData))
		}

		// Try saving to temporary file first
		tempFile, tempErr := saveImageToTempFile(imageData)
		if tempErr != nil {
			log.Printf("Failed to save temp file: %v", tempErr)
			// Continue with direct upload
		} else {
			defer os.Remove(tempFile) // Clean up temp file
			log.Printf("Saved image to temp file: %s", tempFile)
		}

		// Try to upload and send the image properly
		log.Printf("Attempting to upload and send image...")

		// Create thumbnail for WhatsApp
		thumbnailData := imageData
		if len(imageData) > 100*1024 {
			log.Printf("Creating thumbnail for large image...")
			thumb, thumbErr := createThumbnail(imageData)
			if thumbErr == nil {
				thumbnailData = thumb
				log.Printf("Thumbnail created: %d bytes", len(thumbnailData))
			}
		}

		// Upload the full image using whatsmeow.MediaImage constant
		uploaded, uploadErr := WaClient.Upload(ctx, imageData, whatsmeow.MediaImage)
		if uploadErr != nil {
			log.Printf("Failed to upload image: %v", uploadErr)
			// Try alternative methods if upload fails
			err = uploadErr
			if i < maxRetries-1 {
				time.Sleep(time.Duration(i+1) * time.Second)
			}
			continue
		}

		log.Printf("Image uploaded successfully")

		// Create image message following official whatsmeow documentation
		imageMsg := &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:       proto.String(caption),
				Mimetype:      proto.String("image/png"),
				JPEGThumbnail: thumbnailData,
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
			},
		}

		// Send the image message
		_, err = WaClient.SendMessage(ctx, targetJID, imageMsg)
		if err == nil {
			log.Printf("Image sent successfully to %s", targetJID.String())
			return nil
		}

		log.Printf("Failed to send image message (attempt %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	// If all attempts failed, try fallback methods
	log.Printf("All upload attempts failed, trying alternative methods...")
	return sendImageFallback(ctx, targetJID, imageBase64, caption)
}

// Send image using fallback methods when upload fails
func sendImageFallback(ctx context.Context, targetJID types.JID, imageBase64 string, caption string) error {
	// Decode base64 image data
	imageData, decodeErr := base64.StdEncoding.DecodeString(imageBase64)
	if decodeErr != nil {
		return fmt.Errorf("failed to decode base64 image: %v", decodeErr)
	}

	// Try to compress image if it's too large
	compressedImageData := imageData
	compressedBase64 := imageBase64

	if len(imageData) > 50*1024 { // If larger than 50KB
		log.Printf("Image too large (%d bytes), attempting compression...", len(imageData))
		compressed, compressErr := compressImage(imageData)
		if compressErr == nil {
			compressedImageData = compressed
			compressedBase64 = base64.StdEncoding.EncodeToString(compressed)
			log.Printf("Image compressed to %d bytes", len(compressedImageData))

			// If still too large, try more aggressive compression
			if len(compressedImageData) > 30*1024 {
				log.Printf("Still too large (%d bytes), trying aggressive compression...", len(compressedImageData))
				aggressiveCompressed, aggressiveErr := compressImageAggressively(imageData)
				if aggressiveErr == nil {
					compressedImageData = aggressiveCompressed
					compressedBase64 = base64.StdEncoding.EncodeToString(aggressiveCompressed)
					log.Printf("Aggressively compressed to %d bytes", len(compressedImageData))
				} else {
					log.Printf("Failed aggressive compression: %v", aggressiveErr)
				}
			}
		} else {
			log.Printf("Failed to compress image: %v", compressErr)
		}
	}

	// Check if data URL is too long for WhatsApp (limit ~4096 chars)
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", compressedBase64)
	if len(dataURL) > 4000 {
		log.Printf("Data URL too long (%d chars), trying thumbnail approach", len(dataURL))

		// Try creating a very small thumbnail
		thumbnailData, thumbnailErr := createThumbnail(compressedImageData)
		if thumbnailErr == nil {
			thumbnailBase64 := base64.StdEncoding.EncodeToString(thumbnailData)
			thumbnailURL := fmt.Sprintf("data:image/jpeg;base64,%s", thumbnailBase64)

			if len(thumbnailURL) <= 4000 {
				log.Printf("Thumbnail created (%d bytes, %d chars), sending thumbnail", len(thumbnailData), len(thumbnailURL))

				thumbnailMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Thumbnail:*\n%s\n\n*Catatan:* Gambar asli terlalu besar, ini adalah thumbnail kecil.", caption, thumbnailURL)

				_, sendErr := WaClient.SendMessage(ctx, targetJID, &waE2E.Message{
					Conversation: proto.String(thumbnailMessage),
				})

				if sendErr == nil {
					log.Printf("Thumbnail sent successfully to %s", targetJID.String())
					return nil
				}
			}
		}

		// If thumbnail also fails, send fallback message
		log.Printf("Thumbnail also too large, sending fallback message")
		fallbackMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n❌ *Gagal Mengirim Gambar*\n\nGambar berhasil dibuat oleh AI tetapi terlalu besar untuk dikirim melalui WhatsApp.\n\n*Detail:*\n• Ukuran file: %d bytes\n• Data URL: %d karakter\n• Batas WhatsApp: ~4000 karakter\n\n*Solusi:*\n• Gunakan deskripsi yang lebih sederhana\n• Coba prompt yang menghasilkan gambar lebih kecil\n• Contoh: `!img simple cat` atau `!img red circle`", caption, len(compressedImageData), len(dataURL))

		_, sendErr := WaClient.SendMessage(ctx, targetJID, &waE2E.Message{
			Conversation: proto.String(fallbackMessage),
		})

		if sendErr == nil {
			log.Printf("Fallback message sent successfully to %s", targetJID.String())
			return nil
		}
	} else {
		// Send as text message with data URL
		urlMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Data URL:*\n%s\n\n*Catatan:* Upload langsung gagal (error 415), gambar tersedia sebagai data URL di atas.", caption, dataURL)

		_, sendErr := WaClient.SendMessage(ctx, targetJID, &waE2E.Message{
			Conversation: proto.String(urlMessage),
		})

		if sendErr == nil {
			log.Printf("Image sent as data URL successfully to %s", targetJID.String())
			return nil
		}
	}

	return fmt.Errorf("failed to send image using all fallback methods")
}

// Save image data to temporary file
func saveImageToTempFile(imageData []byte) (string, error) {
	// Create temp file
	tempFile, err := ioutil.TempFile("", "whatsapp_image_*.png")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Write image data
	_, err = tempFile.Write(imageData)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// Compress image to reduce size
func compressImage(imageData []byte) ([]byte, error) {
	// Decode PNG image
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %v", err)
	}

	// Create a buffer to write compressed JPEG
	var buf bytes.Buffer

	// Encode as JPEG with quality 70 (good balance between size and quality)
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

// Aggressively compress image to reduce size further
func compressImageAggressively(imageData []byte) ([]byte, error) {
	// Decode PNG image
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %v", err)
	}

	// Create a buffer to write compressed JPEG
	var buf bytes.Buffer

	// Encode as JPEG with quality 30 (very aggressive compression)
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 30})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

// Create a small thumbnail for very large images
func createThumbnail(imageData []byte) ([]byte, error) {
	// Try to decode as JPEG first (since it's already compressed)
	img, err := jpeg.Decode(bytes.NewReader(imageData))
	if err != nil {
		// If not JPEG, try PNG
		img, err = png.Decode(bytes.NewReader(imageData))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
	}

	// Get original dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate thumbnail size (max 64x64 for smaller data URL)
	maxSize := 64
	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = (height * maxSize) / width
	} else {
		newHeight = maxSize
		newWidth = (width * maxSize) / height
	}

	// Ensure minimum size
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	// Create thumbnail image
	thumbnail := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Simple nearest neighbor resize
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := (x * width) / newWidth
			srcY := (y * height) / newHeight
			thumbnail.Set(x, y, img.At(srcX, srcY))
		}
	}

	// Encode as JPEG with very low quality (10 for minimal size)
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 10})
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

// Send image as URL message when direct upload fails
func sendImageAsURL(ctx context.Context, targetJID types.JID, imageBase64 string, caption string) error {
	// Create a data URL for the image
	dataURL := fmt.Sprintf("data:image/png;base64,%s", imageBase64)

	// Send as text message with data URL
	urlMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Data URL:*\n%s\n\n*Catatan:* Upload langsung gagal, gambar tersedia sebagai data URL di atas.", caption, dataURL)

	_, err := WaClient.SendMessage(ctx, targetJID, &waE2E.Message{
		Conversation: proto.String(urlMessage),
	})

	if err != nil {
		return fmt.Errorf("failed to send URL message: %v", err)
	}

	log.Printf("Image sent as URL message to %s", targetJID.String())
	return nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
