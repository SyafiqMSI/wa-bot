package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsmeow-api/services/gemini"
	"whatsmeow-api/services/idx"
	"whatsmeow-api/utils"
	"whatsmeow-api/whatsapp"
)

func handleHelpCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	helpMessage := `[WhatsApp Bot] Bantuan Penggunaan

[Daftar Perintah]

*!help* atau */help*
Menampilkan bantuan dan cara penggunaan bot

*!hallo* atau */hallo*
Menyapa bot dengan ramah

*!fiq [pertanyaan]* atau */fiq [pertanyaan]*
Tanya apa saja ke asisten AI pribadi Fiq

*!groups* atau */groups*
Menampilkan daftar grup yang diikuti bot

*!groups [nama grup]* atau */groups [nama grup]*
Mencari grup berdasarkan nama dan menampilkan ID-nya
Contoh: *!groups Braincore Community*

*!ping* atau */ping*
Cek apakah bot sedang aktif

*!status* atau */status*
Menampilkan status koneksi bot

*!info* atau */info*
Menampilkan informasi tentang bot

*!test* atau */test*
Test apakah bot berfungsi dengan baik

*!echo [teks]* atau */echo [teks]*
Mengulang pesan yang dikirim

*!idx* atau */idx*
Menampilkan data pasar saham IDX hari ini

*!img [deskripsi]* atau */img [deskripsi]*
Membuat gambar AI berdasarkan deskripsi yang diberikan

[Tips]
- Semua perintah bisa menggunakan ! atau /
- Bot akan merespons secara otomatis
- Gunakan perintah di chat pribadi atau grup

[Fiq - Asisten AI]
Fiq adalah asisten pribadi berbasis Google Gemini yang siap membantu Anda dengan berbagai pertanyaan dan tugas sehari-hari.

[Dukungan]
Jika ada pertanyaan, silakan hubungi administrator bot.`

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, helpMessage, 2)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

func handleHalloCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	senderName := "teman"
	if v.Info.PushName != "" {
		senderName = v.Info.PushName
	}

	halloMessage := fmt.Sprintf("[%s] Hallo %s!\n\nSenang bertemu denganmu! Ada yang bisa saya bantu hari ini?\n\nKetik *!help* untuk melihat semua perintah yang tersedia.", "Bot", senderName)

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, halloMessage, 2)
	if err != nil {
		log.Printf("Failed to send hallo message: %v", err)
	}
}

func handlePingCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	pingMessage := "[Ping] Pong! Bot sedang aktif dan siap melayani."

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, pingMessage, 2)
	if err != nil {
		log.Printf("Failed to send ping message: %v", err)
	}
}

func handleStatusCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Bot sedang tidak terhubung ke WhatsApp", 2)
		return
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	statusMessage := fmt.Sprintf(`[Status Bot]

Koneksi WhatsApp: Terhubung
Bot Status: Aktif
Waktu: %s
Uptime: Bot sedang berjalan

Semua sistem berfungsi dengan baik!`, time.Now().In(loc).Format("02 Jan 2006, 15:04:05 WIB"))

	err = utils.SendMessageWithRetry(context.Background(), v.Info.Chat, statusMessage, 2)
	if err != nil {
		log.Printf("Failed to send status message: %v", err)
	}
}

func handleInfoCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	infoMessage := `[Informasi Bot]

Nama: WhatsApp Bot API
Versi: 2.0.0
Developer: WhatsApp Bot Team
Bahasa: Go (Golang)
Platform: WhatsApp Web
Fitur: Auto-reply, Group Management, Message API

Bot ini dibuat untuk memudahkan komunikasi dan otomasi pesan WhatsApp melalui API.`

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, infoMessage, 2)
	if err != nil {
		log.Printf("Failed to send info message: %v", err)
	}
}

func handleTestCommand(v *events.Message) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	testMessage := `[Test Bot Response]

Bot Status: Aktif dan berfungsi dengan baik
Connection: WhatsApp terhubung
Commands: Case insensitive aktif
Web Support: WhatsApp Web didukung

Test berhasil! Bot siap menerima perintah dalam berbagai format:
- huruf BESAR: !HELP, !PING, !STATUS
- huruf kecil: !help, !ping, !status
- Campuran: !HeLp, !PiNg, !StAtUs

Semua format akan dikenali dengan benar!`

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, testMessage, 2)
	if err != nil {
		log.Printf("Failed to send test message: %v", err)
	}
}

func handleEchoCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var echoText string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!echo ") {
		echoText = strings.TrimSpace(originalMessage[6:])
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/echo ") {
		echoText = strings.TrimSpace(originalMessage[6:])
	} else {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	if echoText == "" {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	echoResponse := fmt.Sprintf("[Echo Response]\n\n%s", echoText)

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, echoResponse, 2)
	if err != nil {
		log.Printf("Failed to send echo message: %v", err)
	}
}

func handleGroupsCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var searchName string
	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!groups ") {
		searchName = strings.TrimSpace(originalMessage[8:])
	} else if strings.HasPrefix(lower, "/groups ") {
		searchName = strings.TrimSpace(originalMessage[8:])
	}

	groups, err := whatsapp.Client.GetJoinedGroups(context.Background())
	if err != nil {
		log.Printf("Failed to get joined groups: %v", err)
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Gagal mengambil daftar grup: "+err.Error(), 2)
		return
	}

	if len(groups) == 0 {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Info] Tidak ada grup yang diikuti.", 2)
		return
	}

	if searchName != "" {

		var matchedGroups []*types.GroupInfo
		searchLower := strings.ToLower(searchName)

		for _, group := range groups {
			groupName := group.Name
			if groupName == "" {
				groupName = "Tanpa Nama"
			}

			if strings.Contains(strings.ToLower(groupName), searchLower) {
				matchedGroups = append(matchedGroups, group)
			}
		}

		if len(matchedGroups) == 0 {
			message := fmt.Sprintf("[Pencarian Grup]\n\nTidak ditemukan grup dengan nama \"%s\"\n\nCoba gunakan kata kunci yang lebih umum atau gunakan !groups untuk melihat semua grup", searchName)
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
			return
		}

		message := fmt.Sprintf("[Hasil Pencarian Grup: \"%s\"]\n\nDitemukan %d grup:\n\n", searchName, len(matchedGroups))

		for _, group := range matchedGroups {
			groupName := group.Name
			if groupName == "" {
				groupName = "Tanpa Nama"
			}

			message += fmt.Sprintf("Name: %s\n", groupName)
			message += fmt.Sprintf("JID: %s\n\n", group.JID.String())
		}

		message += "[Tips: Gunakan !groups [nama grup] untuk mencari grup lain]"

		err = utils.SendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
		if err != nil {
			log.Printf("Failed to send groups search result: %v", err)
		}
		return
	}

	message := fmt.Sprintf("[Daftar Grup yang Diikuti] (%d grup)\n\n", len(groups))

	for i, group := range groups {
		if i >= 20 {
			message += fmt.Sprintf("_... dan %d grup lainnya_\n", len(groups)-20)
			break
		}

		groupName := group.Name
		if groupName == "" {
			groupName = "Tanpa Nama"
		}

		message += fmt.Sprintf("Name: %s\n", groupName)
		message += fmt.Sprintf("JID: %s\n", group.JID.String())
	}

	message += "\n[Tips] Gunakan !groups [nama grup] untuk mencari grup tertentu\n"
	message += "Contoh: !groups Braincore Community"

	err = utils.SendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
	if err != nil {
		log.Printf("Failed to send groups list: %v", err)
	}
}

func handleFiqCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var userMessage string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!fiq ") {
		userMessage = strings.TrimSpace(originalMessage[5:])
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/fiq ") {
		userMessage = strings.TrimSpace(originalMessage[5:])
	} else {

		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Fiq - Asisten Pribadi]\n\nHalo! Saya adalah Fiq, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n- !fiq [pertanyaan Anda]\n- !fiq apa kabar?\n- !fiq bantu saya dengan...\n\nContoh: !fiq jelaskan tentang Go programming", 2)
		return
	}

	if userMessage == "" {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Fiq - Asisten Pribadi]\n\nHalo! Saya adalah Fiq, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n- !fiq [pertanyaan Anda]\n- !fiq apa kabar?\n- !fiq bantu saya dengan...\n\nContoh: !fiq jelaskan tentang Go programming", 2)
		return
	}

	utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Fiq] Sedang berpikir...\n\nMohon tunggu sebentar ya, saya sedang memproses permintaan Anda.", 2)

	response, err := gemini.GetGeminiResponseWithMemory(context.Background(), v.Info.Chat.String(), "Fiq", userMessage)
	if err != nil {
		log.Printf("Failed to get Gemini response: %v", err)

		if strings.Contains(err.Error(), "API key not configured") {
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] API_KEY_GEMINI belum dikonfigurasi di environment variable.\n\nSilakan set environment variable API_KEY_GEMINI dengan Google Gemini API key Anda.", 2)
			return
		}

		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Maaf, terjadi kesalahan saat memproses permintaan Anda. Silakan coba lagi nanti.", 2)
		return
	}

	formattedResponse := fmt.Sprintf("[Fiq]\n\n%s\n\n---\n[Ketik !fiq [pertanyaan] untuk bertanya lagi]", response)

	err = utils.SendMessageWithRetry(context.Background(), v.Info.Chat, formattedResponse, 2)
	if err != nil {
		log.Printf("Failed to send Fiq response: %v", err)
	}
}

func handleApikCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var userMessage string
	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!apik ") {
		userMessage = strings.TrimSpace(originalMessage[6:])
	} else if strings.HasPrefix(lower, "/apik ") {
		userMessage = strings.TrimSpace(originalMessage[6:])
	} else {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[!apik - Asisten Pribadi]\n\nHalo! Saya adalah !apik, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n- !apik [pertanyaan Anda]\n- !apik apa kabar?\n- !apik bantu saya dengan...\n\nContoh: !apik jelaskan tentang Go programming", 2)
		return
	}

	if userMessage == "" {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[!apik - Asisten Pribadi]\n\nHalo! Saya adalah !apik, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n- !apik [pertanyaan Anda]\n- !apik apa kabar?\n- !apik bantu saya dengan...\n\nContoh: !apik jelaskan tentang Go programming", 2)
		return
	}

	utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[!apik] Sedang berpikir...\n\nMohon tunggu sebentar ya, saya sedang memproses permintaan Anda.", 2)

	response, err := gemini.GetGeminiResponseWithMemory(context.Background(), v.Info.Chat.String(), "!apik", userMessage)
	if err != nil {
		log.Printf("Failed to get Gemini response (!apik): %v", err)
		if strings.Contains(err.Error(), "API key not configured") {
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] API_KEY_GEMINI belum dikonfigurasi di environment variable.", 2)
			return
		}
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Maaf, terjadi kesalahan saat memproses permintaan Anda. Silakan coba lagi nanti.", 2)
		return
	}

	formattedResponse := fmt.Sprintf("[!apik]\n\n%s\n\n---\n[Ketik !apik [pertanyaan] untuk bertanya lagi]", response)
	if err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, formattedResponse, 2); err != nil {
		log.Printf("Failed to send !apik response: %v", err)
	}
}

func handleIDXCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	log.Printf("IDX command received from %s: %s", v.Info.Sender.String(), originalMessage)

	var targetDate time.Time
	var dateStr string

	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!idx ") {
		dateStr = strings.TrimSpace(originalMessage[5:])
	} else if strings.HasPrefix(lower, "/idx ") {
		dateStr = strings.TrimSpace(originalMessage[5:])
	}

	if dateStr != "" {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.FixedZone("WIB", 7*3600)
		}

		monthMap := map[string]string{
			"januari": "January", "jan": "Jan", "februari": "February", "feb": "Feb",
			"maret": "March", "mar": "Mar", "april": "April", "apr": "Apr",
			"mei": "May", "may": "May", "juni": "June", "jun": "Jun",
			"juli": "July", "jul": "Jul", "agustus": "August", "aug": "Aug",
			"september": "September", "sep": "Sep", "oktober": "October", "oct": "Oct",
			"november": "November", "nov": "Nov", "desember": "December", "dec": "Dec",
		}

		dStr := strings.ToLower(dateStr)
		for indo, eng := range monthMap {
			if strings.Contains(dStr, indo) {
				dStr = strings.ReplaceAll(dStr, indo, eng)
			}
		}

		formats := []string{
			"2 January 2006", "02 January 2006", "2 Jan 2006", "02 Jan 2006",
			"2 January", "02 January", "2 Jan", "02 Jan",
			"02/01/2006", "02-01-2006", "2006-01-02",
		}

		parsed := false
		for _, f := range formats {
			if t, err := time.Parse(f, dStr); err == nil {
				if t.Year() == 0 {
					now := time.Now().In(loc)
					t = time.Date(now.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
				} else {
					t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
				}
				targetDate = t
				parsed = true
				break
			}
		}

		if !parsed {
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Format tanggal tidak dikenali. Contoh: !idx 27 februari 2026", 2)
			return
		}
	} else {
		targetDate = time.Now()
	}

	dateFmt := targetDate.Format("02 Jan 2006")
	loadingMessage := fmt.Sprintf("[IDX] Mengambil data pasar IDX untuk tanggal %s...\n\nSilakan tunggu sebentar...", dateFmt)
	if err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, loadingMessage, 2); err != nil {
		log.Printf("Failed to send loading message: %v", err)
	}

	data, err := idx.GetIDXMarketData(targetDate)
	if err != nil {
		errorMessage := "[Error] Gagal mengambil data pasar IDX. Silakan coba lagi nanti."
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, errorMessage, 2)
		return
	}

	response := idx.FormatIDXResponse(data)
	if err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, response, 2); err != nil {
		log.Printf("Failed to send IDX response: %v", err)
	}
}

func handleImgCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var prompt string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!img ") {
		prompt = strings.TrimSpace(originalMessage[5:])
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/img ") {
		prompt = strings.TrimSpace(originalMessage[5:])
	} else {

		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Generator Gambar AI]\n\nHalo! Saya dapat membuat gambar berdasarkan deskripsi Anda.\n\nCara menggunakan:\n- !img [deskripsi gambar]\n- !img pemandangan gunung dengan matahari terbenam\n- !img kucing lucu bermain di taman\n\nContoh: !img robot futuristik di kota masa depan", 2)
		return
	}

	if prompt == "" {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Generator Gambar AI]\n\nHalo! Saya dapat membuat gambar berdasarkan deskripsi Anda.\n\nCara menggunakan:\n- !img [deskripsi gambar]\n- !img pemandangan gunung dengan matahari terbenam\n- !img kucing lucu bermain di taman\n\nContoh: !img robot futuristik di kota masa depan", 2)
		return
	}

	utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[AI] Sedang membuat gambar...\n\nMohon tunggu sebentar ya, saya sedang membuat gambar berdasarkan deskripsi Anda. Proses ini mungkin membutuhkan waktu 30-60 detik.", 2)

	imageBase64, err := gemini.GetGeminiImage(context.Background(), prompt)
	if err != nil {
		log.Printf("Failed to generate image: %v", err)
		if strings.Contains(err.Error(), "API key not configured") {
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] API_KEY_GEMINI belum dikonfigurasi di environment variable.\n\nSilakan set environment variable API_KEY_GEMINI dengan Google Gemini API key Anda.", 2)
			return
		}
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Quota Gemini Habis\n\nMaaf, quota API Gemini untuk hari ini sudah habis atau rate limit tercapai. Silakan coba lagi nanti (biasanya reset setiap 24 jam) atau upgrade ke paid plan untuk quota lebih besar.", 2)
			return
		}
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Maaf, terjadi kesalahan saat membuat gambar. Silakan coba lagi nanti atau gunakan deskripsi yang lebih sederhana.", 2)
		return
	}

	caption := fmt.Sprintf("[Gambar AI Generated]\n\nPrompt: %s\n\nDibuat menggunakan Gemini 2.0 Flash Preview Image Generation", prompt)

	err = utils.SendImageWithRetry(context.Background(), v.Info.Chat, imageBase64, caption, 3)
	if err != nil {
		log.Printf("Failed to send generated image: %v", err)

		if strings.Contains(err.Error(), "data URL") || strings.Contains(err.Error(), "fallback message") || strings.Contains(err.Error(), "thumbnail") {
			log.Printf("Image sent successfully (as data URL, thumbnail, or fallback)")
			return
		}

		fallbackMessage := fmt.Sprintf("[Gambar Berhasil Dibuat]\n\nPrompt: %s\n\n[Error]\n\nGambar berhasil dibuat oleh AI tetapi gagal dikirim ke WhatsApp. Kemungkinan penyebab:\n- Ukuran file terlalu besar\n- Masalah koneksi\n- Format tidak didukung\n\nSilakan coba lagi dengan deskripsi yang lebih sederhana atau tunggu beberapa saat.", prompt)
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, fallbackMessage, 2)
		return
	}

	log.Printf("Successfully generated and sent image for prompt: %s", prompt)
}

func handleCCTVCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	ownerJidStr := os.Getenv("OWNER_JID")
	if ownerJidStr == "" {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] OWNER_JID belum dikonfigurasi pada server.", 2)
		return
	}

	senderJID := v.Info.Sender.ToNonAD()
	isOwner := false

	owners := strings.Split(ownerJidStr, ",")
	for _, ownerCandidate := range owners {
		ownerCandidate = strings.TrimSpace(ownerCandidate)
		if ownerCandidate == "" {
			continue
		}

		candidateJid := utils.CreateTargetJID(ownerCandidate)

		// Match against several variations of the sender's identifier
		// 1. Raw sender user ID (e.g. 628123456789)
		// 2. The full sender JID string without device ID
		// 3. The raw sender string
		// 4. Specifically match if the owner configuration was provided as a LID (e.g. 202219995570386@lid)
		if senderJID.User == candidateJid.User ||
			senderJID.String() == candidateJid.String() ||
			senderJID.String() == ownerCandidate ||
			v.Info.Sender.User == candidateJid.User ||
			strings.Contains(v.Info.Sender.String(), ownerCandidate) {
			isOwner = true
			break
		}
	}

	// Check if sender is the owner
	if !isOwner {
		log.Printf("[CCTV] Unauthorized access attempt by: %s (Base: %s, User: %s)", v.Info.Sender.String(), senderJID.String(), senderJID.User)
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Anda tidak memiliki izin untuk menggunakan perintah ini.", 2)
		return
	}

	baseURL := os.Getenv("VISERON_BASE_URL")
	camera := os.Getenv("VISERON_DEFAULT_CAMERA")

	if baseURL == "" || camera == "" {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Konfigurasi Viseron (VISERON_BASE_URL, VISERON_DEFAULT_CAMERA) belum lengkap.", 2)
		return
	}

	utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[CCTV] Sedang mengambil gambar dari kamera...", 2)

	// Build the API endpoint to get the latest snapshot
	// Viseron typically provides a latest snapshot endpoint such as /api/v1/camera/camera_1/snapshot
	snapshotURL := fmt.Sprintf("%s/api/v1/camera/%s/snapshot", baseURL, camera)

	imgData, err := fetchBytes(snapshotURL, 15*time.Second)
	if err != nil {
		log.Printf("[CCTV] Failed to fetch manual snapshot: %v", err)
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, fmt.Sprintf("[Error] Gagal mengambil gambar dari CCTV: %v", err), 2)
		return
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	imgBase64 := base64.StdEncoding.EncodeToString(imgData)
	caption := fmt.Sprintf("[CCTV Manual Snapshot]\n\nKamera: %s\nWaktu: %s", camera, time.Now().In(loc).Format("02 Jan 2006, 15:04:05 WIB"))

	err = utils.SendImageWithRetry(context.Background(), v.Info.Chat, imgBase64, caption, 3)
	if err != nil {
		log.Printf("Failed to send manual CCTV snapshot: %v", err)
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[Error] Gagal mengirim gambar CCTV ke WhatsApp.", 2)
	}

	// We can optionally trigger a video clip capture
	// We run it as a goroutine because it takes 30s to record
	go func() {
		utils.SendMessageWithRetry(context.Background(), v.Info.Chat, "[CCTV] Sedang merekam video klip (30 detik)...", 2)
		sendHLSClipToTargets([]string{v.Info.Chat.String()}, baseURL, camera, "Manual Request Video", time.Now())
	}()
}

func handleJIDCommand(v *events.Message, originalMessage string) {
	if !whatsapp.Client.IsConnected() {
		return
	}

	var target string
	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!jid ") {
		target = strings.TrimSpace(originalMessage[5:])
	} else if strings.HasPrefix(lower, "/jid ") {
		target = strings.TrimSpace(originalMessage[5:])
	}

	var response string
	if target == "" {
		// Output sender's own JID and the chat's JID
		response = fmt.Sprintf("[Info JID]\n\nJID Anda: %s\nJID Chat ini: %s", v.Info.Sender.ToNonAD().String(), v.Info.Chat.String())
	} else {
		// Generate JID from the target using the existing utility
		jid := utils.CreateTargetJID(target)
		response = fmt.Sprintf("[Info JID]\n\nInput: %s\nJID Format: %s", target, jid.String())
	}

	err := utils.SendMessageWithRetry(context.Background(), v.Info.Chat, response, 2)
	if err != nil {
		log.Printf("Failed to send JID info: %v", err)
	}
}
