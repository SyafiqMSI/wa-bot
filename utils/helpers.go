package utils

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

	"whatsmeow-api/domain"
	"whatsmeow-api/whatsapp"
)

func HasCommandPrefix(message, command string) bool {
	messageLower := strings.ToLower(message)
	return strings.HasPrefix(messageLower, strings.ToLower(command))
}

func ContainsCommand(message, command string) bool {
	messageLower := strings.ToLower(message)
	return strings.Contains(messageLower, strings.ToLower(command))
}

func GetNotificationTargets() []string {
	targets := os.Getenv("NOTIFICATION_TARGETS")
	if targets == "" {
		return []string{}
	}
	return strings.Split(targets, ",")
}

func GetNoResponseGroups() []string {
	noResponse := os.Getenv("NO_RESPONSE")
	if noResponse == "" {
		return []string{}
	}

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

func ShouldIgnoreGroup(chatJID string) bool {
	noResponseGroups := GetNoResponseGroups()
	if len(noResponseGroups) == 0 {
		return false
	}

	chatJID = strings.TrimSpace(chatJID)

	for _, ignoredJID := range noResponseGroups {
		if strings.TrimSpace(ignoredJID) == chatJID {
			return true
		}
	}
	return false
}

func IsGroupJID(target string) bool {

	return strings.HasSuffix(target, "@g.us")
}

func CreateTargetJID(target string) types.JID {
	target = strings.TrimSpace(target)

	if IsGroupJID(target) {

		jid, err := types.ParseJID(target)
		if err != nil {
			log.Printf("Invalid group JID format: %s, error: %v", target, err)

			return types.JID{}
		}
		return jid
	} else {

		normalizedTarget := NormalizePhoneNumber(target)
		return types.NewJID(normalizedTarget, types.DefaultUserServer)
	}
}

func GetPusherName(payload *domain.GitHubWebhookPayload) string {
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

func GetFileChangesSummary(commit domain.Commit) string {
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

func NormalizePhoneNumber(phone string) string {
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

func SendMessageWithRetry(ctx context.Context, targetJID types.JID, message string, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		_, err = whatsapp.Client.SendMessage(ctx, targetJID, &waE2E.Message{
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

func GetMessageText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	if txt := msg.GetConversation(); txt != "" {
		return txt
	}

	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if t := ext.GetText(); t != "" {
			return t
		}
	}

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

	if ep := msg.GetEphemeralMessage(); ep != nil {
		return GetMessageText(ep.GetMessage())
	}
	if dv := msg.GetDeviceSentMessage(); dv != nil {
		return GetMessageText(dv.GetMessage())
	}

	return ""
}

func SendImageWithRetry(ctx context.Context, targetJID types.JID, imageBase64 string, caption string, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {

		imageData, decodeErr := base64.StdEncoding.DecodeString(imageBase64)
		if decodeErr != nil {
			return fmt.Errorf("failed to decode base64 image: %v", decodeErr)
		}

		log.Printf("Image data size: %d bytes", len(imageData))

		if len(imageData) > 15*1024*1024 {
			return fmt.Errorf("image too large: %d bytes (max 15MB)", len(imageData))
		}

		tempFile, tempErr := SaveImageToTempFile(imageData)
		if tempErr != nil {
			log.Printf("Failed to save temp file: %v", tempErr)

		} else {
			defer os.Remove(tempFile)
			log.Printf("Saved image to temp file: %s", tempFile)
		}

		log.Printf("Attempting to upload and send image...")

		thumbnailData := imageData
		if len(imageData) > 100*1024 {
			log.Printf("Creating thumbnail for large image...")
			thumb, thumbErr := CreateThumbnail(imageData)
			if thumbErr == nil {
				thumbnailData = thumb
				log.Printf("Thumbnail created: %d bytes", len(thumbnailData))
			}
		}

		uploaded, uploadErr := whatsapp.Client.Upload(ctx, imageData, whatsmeow.MediaImage)
		if uploadErr != nil {
			log.Printf("Failed to upload image: %v", uploadErr)

			err = uploadErr
			if i < maxRetries-1 {
				time.Sleep(time.Duration(i+1) * time.Second)
			}
			continue
		}

		log.Printf("Image uploaded successfully")

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

		_, err = whatsapp.Client.SendMessage(ctx, targetJID, imageMsg)
		if err == nil {
			log.Printf("Image sent successfully to %s", targetJID.String())
			return nil
		}

		log.Printf("Failed to send image message (attempt %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	log.Printf("All upload attempts failed, trying alternative methods...")
	return SendImageFallback(ctx, targetJID, imageBase64, caption)
}

func SendImageFallback(ctx context.Context, targetJID types.JID, imageBase64 string, caption string) error {

	imageData, decodeErr := base64.StdEncoding.DecodeString(imageBase64)
	if decodeErr != nil {
		return fmt.Errorf("failed to decode base64 image: %v", decodeErr)
	}

	compressedImageData := imageData
	compressedBase64 := imageBase64

	if len(imageData) > 50*1024 {
		log.Printf("Image too large (%d bytes), attempting compression...", len(imageData))
		compressed, compressErr := CompressImage(imageData)
		if compressErr == nil {
			compressedImageData = compressed
			compressedBase64 = base64.StdEncoding.EncodeToString(compressed)
			log.Printf("Image compressed to %d bytes", len(compressedImageData))

			if len(compressedImageData) > 30*1024 {
				log.Printf("Still too large (%d bytes), trying aggressive compression...", len(compressedImageData))
				aggressiveCompressed, aggressiveErr := CompressImageAggressively(imageData)
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

	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", compressedBase64)
	if len(dataURL) > 4000 {
		log.Printf("Data URL too long (%d chars), trying thumbnail approach", len(dataURL))

		thumbnailData, thumbnailErr := CreateThumbnail(compressedImageData)
		if thumbnailErr == nil {
			thumbnailBase64 := base64.StdEncoding.EncodeToString(thumbnailData)
			thumbnailURL := fmt.Sprintf("data:image/jpeg;base64,%s", thumbnailBase64)

			if len(thumbnailURL) <= 4000 {
				log.Printf("Thumbnail created (%d bytes, %d chars), sending thumbnail", len(thumbnailData), len(thumbnailURL))

				thumbnailMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Thumbnail:*\n%s\n\n*Catatan:* Gambar asli terlalu besar, ini adalah thumbnail kecil.", caption, thumbnailURL)

				_, sendErr := whatsapp.Client.SendMessage(ctx, targetJID, &waE2E.Message{
					Conversation: proto.String(thumbnailMessage),
				})

				if sendErr == nil {
					log.Printf("Thumbnail sent successfully to %s", targetJID.String())
					return nil
				}
			}
		}

		log.Printf("Thumbnail also too large, sending fallback message")
		fallbackMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n❌ *Gagal Mengirim Gambar*\n\nGambar berhasil dibuat oleh AI tetapi terlalu besar untuk dikirim melalui WhatsApp.\n\n*Detail:*\n• Ukuran file: %d bytes\n• Data URL: %d karakter\n• Batas WhatsApp: ~4000 karakter\n\n*Solusi:*\n• Gunakan deskripsi yang lebih sederhana\n• Coba prompt yang menghasilkan gambar lebih kecil\n• Contoh: `!img simple cat` atau `!img red circle`", caption, len(compressedImageData), len(dataURL))

		_, sendErr := whatsapp.Client.SendMessage(ctx, targetJID, &waE2E.Message{
			Conversation: proto.String(fallbackMessage),
		})

		if sendErr == nil {
			log.Printf("Fallback message sent successfully to %s", targetJID.String())
			return nil
		}
	} else {

		urlMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Data URL:*\n%s\n\n*Catatan:* Upload langsung gagal (error 415), gambar tersedia sebagai data URL di atas.", caption, dataURL)

		_, sendErr := whatsapp.Client.SendMessage(ctx, targetJID, &waE2E.Message{
			Conversation: proto.String(urlMessage),
		})

		if sendErr == nil {
			log.Printf("Image sent as data URL successfully to %s", targetJID.String())
			return nil
		}
	}

	return fmt.Errorf("failed to send image using all fallback methods")
}

func SaveImageToTempFile(imageData []byte) (string, error) {

	tempFile, err := ioutil.TempFile("", "whatsapp_image_*.png")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = tempFile.Write(imageData)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func CompressImage(imageData []byte) ([]byte, error) {

	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %v", err)
	}

	var buf bytes.Buffer

	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

func CompressImageAggressively(imageData []byte) ([]byte, error) {

	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %v", err)
	}

	var buf bytes.Buffer

	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 30})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

func CreateThumbnail(imageData []byte) ([]byte, error) {

	img, err := jpeg.Decode(bytes.NewReader(imageData))
	if err != nil {

		img, err = png.Decode(bytes.NewReader(imageData))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	maxSize := 64
	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = (height * maxSize) / width
	} else {
		newHeight = maxSize
		newWidth = (width * maxSize) / height
	}

	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	thumbnail := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := (x * width) / newWidth
			srcY := (y * height) / newHeight
			thumbnail.Set(x, y, img.At(srcX, srcY))
		}
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 10})
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

func SendImageAsURL(ctx context.Context, targetJID types.JID, imageBase64 string, caption string) error {

	dataURL := fmt.Sprintf("data:image/png;base64,%s", imageBase64)

	urlMessage := fmt.Sprintf("🎨 *Gambar AI Generated*\n\n%s\n\n📎 *Data URL:*\n%s\n\n*Catatan:* Upload langsung gagal, gambar tersedia sebagai data URL di atas.", caption, dataURL)

	_, err := whatsapp.Client.SendMessage(ctx, targetJID, &waE2E.Message{
		Conversation: proto.String(urlMessage),
	})

	if err != nil {
		return fmt.Errorf("failed to send URL message: %v", err)
	}

	log.Printf("Image sent as URL message to %s", targetJID.String())
	return nil
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
