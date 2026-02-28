package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"whatsmeow-api/domain"
	"whatsmeow-api/utils"
	"whatsmeow-api/whatsapp"
)

var (
	viseronCooldownMu   sync.Mutex
	viseronLastNotified = make(map[string]time.Time)
)

func getCooldownDuration() time.Duration {
	val := os.Getenv("VISERON_COOLDOWN_SECONDS")
	if val == "" {
		return 60 * time.Second
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs < 0 {
		return 60 * time.Second
	}
	return time.Duration(secs) * time.Second
}

func checkCooldown(camera, eventType string) bool {
	cooldown := getCooldownDuration()
	if cooldown == 0 {
		return true
	}
	key := camera + ":" + eventType
	viseronCooldownMu.Lock()
	defer viseronCooldownMu.Unlock()
	last, exists := viseronLastNotified[key]
	if exists && time.Since(last) < cooldown {
		remaining := cooldown - time.Since(last)
		log.Printf("[cooldown] %s/%s: %.0f detik tersisa", camera, eventType, remaining.Seconds())
		return false
	}
	viseronLastNotified[key] = time.Now()
	return true
}

func getViseronTarget() []string {
	raw := os.Getenv("VISERON_TARGET")
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func deriveBaseURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	idx := strings.Index(rawURL, "//")
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+2:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return rawURL
	}
	return rawURL[:idx+2+slashIdx]
}

func formatViseronCaption(payload *domain.ViseronPayload) string {
	now := time.Now().Format("02 Jan 2006, 15:04:05")
	if payload.TriggerTime != "" {
		now = payload.TriggerTime
	}
	cameraID := payload.Camera
	if payload.CameraName != "" {
		cameraID = payload.CameraName
	}
	if cameraID == "" {
		cameraID = "Unknown Camera"
	}

	switch payload.EventType {
	case "motion_detected":
		return strings.Join([]string{
			"*[Viseron] Gerakan Terdeteksi*",
			fmt.Sprintf("Kamera : %s", cameraID),
			fmt.Sprintf("Waktu  : %s", now),
			"",
			"Terdeteksi adanya pergerakan di area kamera.",
		}, "\n")
	default:
		lines := []string{
			"*[Viseron] Objek Terdeteksi*",
			fmt.Sprintf("Kamera : %s", cameraID),
			fmt.Sprintf("Waktu  : %s", now),
		}
		if len(payload.Objects) > 0 {
			lines = append(lines, "Objek  :")
			for _, obj := range payload.Objects {
				lines = append(lines, fmt.Sprintf("  - %s (%.0f%%)", obj.Label, obj.Confidence*100))
			}
		} else {
			lines = append(lines, "Event  : object_detected")
		}
		return strings.Join(lines, "\n")
	}
}

func fetchBytes(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response from %s", url)
	}
	return data, nil
}

func captureHLSClip(hlsURL string, durationSec int) (string, error) {
	outFile := filepath.Join(os.TempDir(), fmt.Sprintf("viseron_clip_%d.mp4", time.Now().UnixNano()))

	args := []string{
		"-y",
		"-allowed_extensions", "ALL",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-i", hlsURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c", "copy",
		"-movflags", "+faststart",
		outFile,
	}
	log.Printf("[ffmpeg] HLS clip: duration=%ds url=%s", durationSec, hlsURL)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ffmpeg] copy failed: %v -- retrying with re-encode\nOutput: %s", err, string(output))

		args2 := []string{
			"-y",
			"-allowed_extensions", "ALL",
			"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
			"-i", hlsURL,
			"-t", fmt.Sprintf("%d", durationSec),
			"-c:v", "libx264", "-preset", "fast", "-crf", "28",
			"-c:a", "aac", "-b:a", "128k",
			"-movflags", "+faststart",
			outFile,
		}
		cmd2 := exec.Command("ffmpeg", args2...)
		output2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return "", fmt.Errorf("ffmpeg re-encode also failed: %v\nOutput: %s", err2, string(output2))
		}
	}

	info, err := os.Stat(outFile)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("ffmpeg produced empty/missing file: %s", outFile)
	}
	log.Printf("[ffmpeg] clip ready: %s (%.1f KB)", outFile, float64(info.Size())/1024)
	return outFile, nil
}

func sendVideoToJID(ctx context.Context, targetJID types.JID, videoData []byte, caption string) error {
	if len(videoData) == 0 {
		return fmt.Errorf("video data is empty")
	}
	const maxVideoSize = 63 * 1024 * 1024
	if len(videoData) > maxVideoSize {
		return fmt.Errorf("video too large: %d bytes (max 63MB)", len(videoData))
	}

	log.Printf("[video] uploading %d bytes to WhatsApp...", len(videoData))
	uploaded, err := whatsapp.Client.Upload(ctx, videoData, whatsmeow.MediaVideo)
	if err != nil {
		return fmt.Errorf("video upload failed: %v", err)
	}

	videoMsg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String("video/mp4"),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		},
	}

	_, err = whatsapp.Client.SendMessage(ctx, targetJID, videoMsg)
	if err != nil {
		return fmt.Errorf("send video message failed: %v", err)
	}
	log.Printf("[video] sent to %s", targetJID.String())
	return nil
}

func sendSnapshotToTargets(ctx context.Context, targets []string, snapshotURL, caption string) {
	if snapshotURL == "" {
		log.Printf("[snapshot] no snapshot_url, sending text only")
		for _, target := range targets {
			jid := utils.CreateTargetJID(target)
			if !jid.IsEmpty() {
				_ = utils.SendMessageWithRetry(ctx, jid, caption, 3)
			}
		}
		return
	}

	imgData, err := fetchBytes(snapshotURL, 10*time.Second)
	if err != nil {
		log.Printf("[snapshot] fetch failed: %v -- text fallback", err)
		for _, target := range targets {
			jid := utils.CreateTargetJID(target)
			if !jid.IsEmpty() {
				_ = utils.SendMessageWithRetry(ctx, jid, caption, 3)
			}
		}
		return
	}

	imgBase64 := base64.StdEncoding.EncodeToString(imgData)
	for i, target := range targets {
		jid := utils.CreateTargetJID(target)
		if jid.IsEmpty() {
			continue
		}
		log.Printf("[snapshot] sending to %s", target)
		if err := utils.SendImageWithRetry(ctx, jid, imgBase64, caption, 3); err != nil {
			log.Printf("[snapshot] image to %s failed: %v -- text fallback", target, err)
			_ = utils.SendMessageWithRetry(ctx, jid, caption, 3)
		}
		if i < len(targets)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func sendHLSClipToTargets(targets []string, baseURL, camera, caption string, eventTime time.Time) {
	if baseURL == "" || camera == "" {
		log.Printf("[video] sendHLSClipToTargets: missing baseURL (%q) or camera (%q)", baseURL, camera)
		return
	}

	startTS := eventTime.Add(-5 * time.Second).Unix()
	endTS := eventTime.Add(20 * time.Second).Unix()
	hlsURL := fmt.Sprintf("%s/api/v1/hls/%s/index.m3u8?start_timestamp=%d&end_timestamp=%d",
		baseURL, camera, startTS, endTS)

	waitUntil := eventTime.Add(25 * time.Second)
	if remaining := time.Until(waitUntil); remaining > 0 {
		log.Printf("[video] waiting %v for HLS segments to be written...", remaining.Round(time.Second))
		time.Sleep(remaining)
	}

	log.Printf("[video] capturing HLS clip: %s", hlsURL)
	clipPath, err := captureHLSClip(hlsURL, 30)
	if err != nil {
		log.Printf("[video] HLS clip capture failed: %v", err)
		return
	}
	defer os.Remove(clipPath)

	videoData, err := os.ReadFile(clipPath)
	if err != nil {
		log.Printf("[video] failed to read clip: %v", err)
		return
	}

	videoCaption := fmt.Sprintf("[Video] Event Clip\n%s", caption)
	ctx := context.Background()

	for i, target := range targets {
		jid := utils.CreateTargetJID(target)
		if jid.IsEmpty() {
			continue
		}
		log.Printf("[video] sending HLS clip to %s", target)
		if err := sendVideoToJID(ctx, jid, videoData, videoCaption); err != nil {
			log.Printf("[video] send to %s failed: %v", target, err)
		}
		if i < len(targets)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

func handleViseronWebhook(w http.ResponseWriter, r *http.Request) {
	log.Printf("[viseron] webhook received: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read body"})
		return
	}
	defer r.Body.Close()

	log.Printf("[viseron] payload (%d bytes): %s", len(body), string(body))

	var payload domain.ViseronPayload
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("[viseron] JSON parse error: %v", err)
		}
	}
	payload.ViseronBaseURL = deriveBaseURL(payload.SnapshotURL)
	log.Printf("[viseron] baseURL=%s camera=%s eventType=%s", payload.ViseronBaseURL, payload.Camera, payload.EventType)

	if !whatsapp.Client.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WhatsApp not connected"})
		return
	}

	targets := getViseronTarget()
	if len(targets) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "VISERON_TARGET not configured"})
		return
	}

	caption := formatViseronCaption(&payload)

	sendVideo := payload.ViseronBaseURL != "" && payload.Camera != ""

	if !checkCooldown(payload.Camera, payload.EventType) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "cooldown"})
		return
	}

	go sendSnapshotToTargets(context.Background(), targets, payload.SnapshotURL, caption)

	if sendVideo {
		baseURL := payload.ViseronBaseURL
		camera := payload.Camera
		eventTime := time.Now()
		go sendHLSClipToTargets(targets, baseURL, camera, caption, eventTime)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "accepted",
		"event_type":    payload.EventType,
		"total_targets": len(targets),
		"sending_image": payload.SnapshotURL != "",
		"sending_video": sendVideo,
	})
}

func handleViseronDebug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	baseURL := r.URL.Query().Get("base")
	camera := r.URL.Query().Get("camera")
	if baseURL == "" || camera == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "query params 'base' and 'camera' are required",
			"example": "/viseron-debug?base=http://172.24.87.44:1000&camera=camera_1",
		})
		return
	}

	apiURL := fmt.Sprintf("%s/api/v1/recordings/%s?latest", baseURL, camera)
	log.Printf("[debug] fetching %s", apiURL)

	data, err := fetchBytes(apiURL, 15*time.Second)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "url": apiURL})
		return
	}

	hlsURL := fmt.Sprintf("%s/api/v1/hls/%s/index.m3u8", baseURL, camera)
	hlsData, hlsErr := fetchBytes(hlsURL, 10*time.Second)
	hlsStatus := "ok"
	if hlsErr != nil {
		hlsStatus = hlsErr.Error()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"recordings_api_url":  apiURL,
		"recordings_response": json.RawMessage(data),
		"hls_url":             hlsURL,
		"hls_status":          hlsStatus,
		"hls_bytes":           len(hlsData),
	})
}
