package handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Handle help command from WhatsApp message
func handleHelpCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	helpMessage := `🤖 *WhatsApp Bot - Bantuan Penggunaan*

*📋 Daftar Perintah:*

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

*💡 Tips:*
- Semua perintah bisa menggunakan ! atau /
- Bot akan merespons secara otomatis
- Gunakan perintah di chat pribadi atau grup

*🤖 Fiq - Asisten AI:*
Fiq adalah asisten pribadi berbasis Google Gemini yang siap membantu Anda dengan berbagai pertanyaan dan tugas sehari-hari.

*📞 Dukungan:*
Jika ada pertanyaan, silakan hubungi administrator bot.`

	// Send response
	err := sendMessageWithRetry(context.Background(), v.Info.Chat, helpMessage, 2)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

// Handle hallo command from WhatsApp message
func handleHalloCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	senderName := "teman"
	if v.Info.PushName != "" {
		senderName = v.Info.PushName
	}

	halloMessage := fmt.Sprintf("👋 Hallo %s! 😊\n\nSenang bertemu denganmu! Ada yang bisa saya bantu hari ini?\n\nKetik *!help* untuk melihat semua perintah yang tersedia.", senderName)

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, halloMessage, 2)
	if err != nil {
		log.Printf("Failed to send hallo message: %v", err)
	}
}

// Handle ping command from WhatsApp message
func handlePingCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	pingMessage := "🏓 Pong! Bot sedang aktif dan siap melayani. ⚡"

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, pingMessage, 2)
	if err != nil {
		log.Printf("Failed to send ping message: %v", err)
	}
}

// Handle status command from WhatsApp message
func handleStatusCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ Bot sedang tidak terhubung ke WhatsApp", 2)
		return
	}

	statusMessage := fmt.Sprintf(`📊 *Status Bot*

✅ *Koneksi WhatsApp:* Terhubung
🤖 *Bot Status:* Aktif
⏰ *Waktu:* %s
🔄 *Uptime:* Bot sedang berjalan

Semua sistem berfungsi dengan baik!`, time.Now().Format("02 Jan 2006, 15:04:05 WIB"))

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, statusMessage, 2)
	if err != nil {
		log.Printf("Failed to send status message: %v", err)
	}
}

// Handle info command from WhatsApp message
func handleInfoCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	infoMessage := `ℹ️ *Informasi Bot*

🤖 *Nama:* WhatsApp Bot API
📝 *Versi:* 2.0.0
👨‍💻 *Developer:* WhatsApp Bot Team
🌐 *Bahasa:* Go (Golang)
📱 *Platform:* WhatsApp Web
⚙️ *Fitur:* Auto-reply, Group Management, Message API

Bot ini dibuat untuk memudahkan komunikasi dan otomasi pesan WhatsApp melalui API.`

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, infoMessage, 2)
	if err != nil {
		log.Printf("Failed to send info message: %v", err)
	}
}

// Handle test command from WhatsApp message
func handleTestCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	testMessage := `🧪 *Test Bot Response*

✅ *Bot Status:* Aktif dan berfungsi dengan baik
✅ *Connection:* WhatsApp terhubung
✅ *Commands:* Case insensitive aktif
✅ *Web Support:* WhatsApp Web didukung

*Test berhasil!* Bot siap menerima perintah dalam berbagai format:
• huruf BESAR: !HELP, !PING, !STATUS
• huruf kecil: !help, !ping, !status
• Campuran: !HeLp, !PiNg, !StAtUs

Semua format akan dikenali dengan benar! 🎉`

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, testMessage, 2)
	if err != nil {
		log.Printf("Failed to send test message: %v", err)
	}
}

// Handle echo command from WhatsApp message
func handleEchoCommand(v *events.Message, originalMessage string) {
	if !WaClient.IsConnected() {
		return
	}

	// Remove the command prefix and get the text to echo
	var echoText string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!echo ") {
		echoText = strings.TrimSpace(originalMessage[6:]) // Remove "!echo "
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/echo ") {
		echoText = strings.TrimSpace(originalMessage[6:]) // Remove "/echo "
	} else {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	if echoText == "" {
		echoText = "Silakan berikan teks setelah perintah echo. Contoh: !echo Halo Dunia"
	}

	echoResponse := fmt.Sprintf("🔊 *Echo Response:*\n\n%s", echoText)

	err := sendMessageWithRetry(context.Background(), v.Info.Chat, echoResponse, 2)
	if err != nil {
		log.Printf("Failed to send echo message: %v", err)
	}
}

// Handle groups command from WhatsApp message
func handleGroupsCommand(v *events.Message, originalMessage string) {
	if !WaClient.IsConnected() {
		return
	}

	// Extract group name after "!groups " or "/groups "
	var searchName string
	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!groups ") {
		searchName = strings.TrimSpace(originalMessage[8:]) // Remove "!groups "
	} else if strings.HasPrefix(lower, "/groups ") {
		searchName = strings.TrimSpace(originalMessage[8:]) // Remove "/groups "
	}

	// Get all groups
	groups, err := WaClient.GetJoinedGroups(context.Background())
	if err != nil {
		log.Printf("Failed to get joined groups: %v", err)
		sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ Gagal mengambil daftar grup: "+err.Error(), 2)
		return
	}

	if len(groups) == 0 {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "📝 Tidak ada grup yang diikuti.", 2)
		return
	}

	// If search name provided, filter groups
	if searchName != "" {
		// Search for groups matching the name (case-insensitive, partial match)
		var matchedGroups []*types.GroupInfo
		searchLower := strings.ToLower(searchName)

		for _, group := range groups {
			groupName := group.Name
			if groupName == "" {
				groupName = "Tanpa Nama"
			}

			// Case-insensitive partial match
			if strings.Contains(strings.ToLower(groupName), searchLower) {
				matchedGroups = append(matchedGroups, group)
			}
		}

		if len(matchedGroups) == 0 {
			message := fmt.Sprintf("🔍 *Pencarian Grup*\n\n❌ Tidak ditemukan grup dengan nama \"%s\"\n\n💡 _Coba gunakan kata kunci yang lebih umum atau gunakan `!groups` untuk melihat semua grup_", searchName)
			sendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
			return
		}

		// Format matched groups
		message := fmt.Sprintf("🔍 *Hasil Pencarian Grup: \"%s\"*\n\n", searchName)
		message += fmt.Sprintf("📊 Ditemukan %d grup:\n\n", len(matchedGroups))

		for _, group := range matchedGroups {
			groupName := group.Name
			if groupName == "" {
				groupName = "Tanpa Nama"
			}

			message += fmt.Sprintf("🏷️ *%s*\n", groupName)
			message += fmt.Sprintf("🆔 `%s`\n\n", group.JID.String())
		}

		message += "💡 _Gunakan `!groups [nama grup]` untuk mencari grup lain_"

		// Send response
		err = sendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
		if err != nil {
			log.Printf("Failed to send groups search result: %v", err)
		}
		return
	}

	// No search name, show all groups
	message := fmt.Sprintf("📋 *Daftar Grup yang Diikuti* (%d grup)\n\n", len(groups))

	for i, group := range groups {
		if i >= 20 { // Limit to 20 groups to avoid message being too long
			message += fmt.Sprintf("_... dan %d grup lainnya_\n", len(groups)-20)
			break
		}

		groupName := group.Name
		if groupName == "" {
			groupName = "Tanpa Nama"
		}

		message += fmt.Sprintf("🏷️ *%s*\n", groupName)
		message += fmt.Sprintf("🆔 `%s`\n", group.JID.String())
	}

	message += "\n💡 _Gunakan `!groups [nama grup]` untuk mencari grup tertentu_\n"
	message += "💡 _Contoh: `!groups Braincore Community`_"

	// Send response
	err = sendMessageWithRetry(context.Background(), v.Info.Chat, message, 2)
	if err != nil {
		log.Printf("Failed to send groups list: %v", err)
	}
}

// Handle fiq command - Gemini AI assistant
func handleFiqCommand(v *events.Message, originalMessage string) {
	if !WaClient.IsConnected() {
		return
	}

	// Extract message after "!fiq " or "/fiq "
	var userMessage string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!fiq ") {
		userMessage = strings.TrimSpace(originalMessage[5:]) // Remove "!fiq "
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/fiq ") {
		userMessage = strings.TrimSpace(originalMessage[5:]) // Remove "/fiq "
	} else {
		// If no message provided, send help
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *Fiq - Asisten Pribadi*\n\nHalo! Saya adalah Fiq, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n• `!fiq [pertanyaan Anda]`\n• `!fiq apa kabar?`\n• `!fiq bantu saya dengan...`\n\nContoh: `!fiq jelaskan tentang Go programming`", 2)
		return
	}

	if userMessage == "" {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *Fiq - Asisten Pribadi*\n\nHalo! Saya adalah Fiq, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n• `!fiq [pertanyaan Anda]`\n• `!fiq apa kabar?`\n• `!fiq bantu saya dengan...`\n\nContoh: `!fiq jelaskan tentang Go programming`", 2)
		return
	}

	// Send thinking message first
	sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *Fiq sedang berpikir...*\n\nMohon tunggu sebentar ya, saya sedang memproses permintaan Anda.", 2)

	// Get response from Gemini with memory using assistant name "Fiq"
	response, err := GetGeminiResponseWithMemory(context.Background(), v.Info.Chat.String(), "Fiq", userMessage)
	if err != nil {
		log.Printf("Failed to get Gemini response: %v", err)

		// Check if API key is not configured
		if strings.Contains(err.Error(), "API key not configured") {
			sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Error:* API_KEY_GEMINI belum dikonfigurasi di environment variable.\n\nSilakan set environment variable API_KEY_GEMINI dengan Google Gemini API key Anda.", 2)
			return
		}

		sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Maaf,* terjadi kesalahan saat memproses permintaan Anda. Silakan coba lagi nanti.", 2)
		return
	}

	// Format response
	formattedResponse := fmt.Sprintf("🤖 *Fiq - Jawaban untuk Anda:*\n\n%s\n\n---\n💡 _Ada yang bisa saya bantu lagi? Ketik `!fiq [pertanyaan]`_", response)

	// Send response
	err = sendMessageWithRetry(context.Background(), v.Info.Chat, formattedResponse, 2)
	if err != nil {
		log.Printf("Failed to send Fiq response: %v", err)
	}
}

// Handle apik command - Gemini AI assistant with name "!apik"
func handleApikCommand(v *events.Message, originalMessage string) {
	if !WaClient.IsConnected() {
		return
	}

	// Extract message after "!apik " or "/apik "
	var userMessage string
	lower := strings.ToLower(originalMessage)
	if strings.HasPrefix(lower, "!apik ") {
		userMessage = strings.TrimSpace(originalMessage[6:])
	} else if strings.HasPrefix(lower, "/apik ") {
		userMessage = strings.TrimSpace(originalMessage[6:])
	} else {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *!apik - Asisten Pribadi*\n\nHalo! Saya adalah !apik, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n• `!apik [pertanyaan Anda]`\n• `!apik apa kabar?`\n• `!apik bantu saya dengan...`\n\nContoh: `!apik jelaskan tentang Go programming`", 2)
		return
	}

	if userMessage == "" {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *!apik - Asisten Pribadi*\n\nHalo! Saya adalah !apik, asisten pribadi Anda yang siap membantu.\n\nCara menggunakan:\n• `!apik [pertanyaan Anda]`\n• `!apik apa kabar?`\n• `!apik bantu saya dengan...`\n\nContoh: `!apik jelaskan tentang Go programming`", 2)
		return
	}

	// Send thinking message first
	sendMessageWithRetry(context.Background(), v.Info.Chat, "🤖 *!apik sedang berpikir...*\n\nMohon tunggu sebentar ya, saya sedang memproses permintaan Anda.", 2)

	// Get response from Gemini with memory using assistant name "!apik"
	response, err := GetGeminiResponseWithMemory(context.Background(), v.Info.Chat.String(), "!apik", userMessage)
	if err != nil {
		log.Printf("Failed to get Gemini response (!apik): %v", err)
		if strings.Contains(err.Error(), "API key not configured") {
			sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Error:* API_KEY_GEMINI belum dikonfigurasi di environment variable.", 2)
			return
		}
		sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Maaf,* terjadi kesalahan saat memproses permintaan Anda. Silakan coba lagi nanti.", 2)
		return
	}

	formattedResponse := fmt.Sprintf("🤖 *!apik - Jawaban untuk Anda:*\n\n%s\n\n---\n💡 _Ada yang bisa saya bantu lagi? Ketik `!apik [pertanyaan]`_", response)
	if err := sendMessageWithRetry(context.Background(), v.Info.Chat, formattedResponse, 2); err != nil {
		log.Printf("Failed to send !apik response: %v", err)
	}
}

// Handle IDX command from WhatsApp message
func handleIDXCommand(v *events.Message) {
	if !WaClient.IsConnected() {
		return
	}

	log.Printf("📊 IDX command received from %s", v.Info.Sender.String())

	// Send loading message
	loadingMessage := "🔄 *Mengambil data pasar IDX...*\n\nSilakan tunggu sebentar..."
	if err := sendMessageWithRetry(context.Background(), v.Info.Chat, loadingMessage, 2); err != nil {
		log.Printf("Failed to send loading message: %v", err)
	}

	// Fetch IDX data
	data, err := GetIDXMarketData()
	if err != nil {
		log.Printf("❌ Error fetching IDX data: %v", err)
		errorMessage := "❌ *Error:* Gagal mengambil data pasar IDX. Silakan coba lagi nanti."
		sendMessageWithRetry(context.Background(), v.Info.Chat, errorMessage, 2)
		return
	}

	// Format and send response
	response := FormatIDXResponse(data)
	if err := sendMessageWithRetry(context.Background(), v.Info.Chat, response, 2); err != nil {
		log.Printf("Failed to send IDX response: %v", err)
	}
}

// Handle img command - Generate image using Gemini 2.5 Flash Image
func handleImgCommand(v *events.Message, originalMessage string) {
	if !WaClient.IsConnected() {
		return
	}

	// Extract prompt after "!img " or "/img "
	var prompt string
	if strings.HasPrefix(strings.ToLower(originalMessage), "!img ") {
		prompt = strings.TrimSpace(originalMessage[5:]) // Remove "!img "
	} else if strings.HasPrefix(strings.ToLower(originalMessage), "/img ") {
		prompt = strings.TrimSpace(originalMessage[5:]) // Remove "/img "
	} else {
		// If no prompt provided, send help
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🎨 *Generator Gambar AI*\n\nHalo! Saya dapat membuat gambar berdasarkan deskripsi Anda.\n\nCara menggunakan:\n• `!img [deskripsi gambar]`\n• `!img pemandangan gunung dengan matahari terbenam`\n• `!img kucing lucu bermain di taman`\n\nContoh: `!img robot futuristik di kota masa depan`", 2)
		return
	}

	if prompt == "" {
		sendMessageWithRetry(context.Background(), v.Info.Chat, "🎨 *Generator Gambar AI*\n\nHalo! Saya dapat membuat gambar berdasarkan deskripsi Anda.\n\nCara menggunakan:\n• `!img [deskripsi gambar]`\n• `!img pemandangan gunung dengan matahari terbenam`\n• `!img kucing lucu bermain di taman`\n\nContoh: `!img robot futuristik di kota masa depan`", 2)
		return
	}

	// Send generating message first
	sendMessageWithRetry(context.Background(), v.Info.Chat, "🎨 *Sedang membuat gambar...*\n\nMohon tunggu sebentar ya, saya sedang membuat gambar berdasarkan deskripsi Anda. Proses ini mungkin membutuhkan waktu 30-60 detik.", 2)

	// Generate image using Gemini 2.5 Flash Image
	imageBase64, err := GetGeminiImage(context.Background(), prompt)
	if err != nil {
		log.Printf("Failed to generate image: %v", err)
		if strings.Contains(err.Error(), "API key not configured") {
			sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Error:* API_KEY_GEMINI belum dikonfigurasi di environment variable.\n\nSilakan set environment variable API_KEY_GEMINI dengan Google Gemini API key Anda.", 2)
			return
		}
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			sendMessageWithRetry(context.Background(), v.Info.Chat, "⏳ *Quota Gemini Habis*\n\nMaaf, quota API Gemini untuk hari ini sudah habis atau rate limit tercapai. Silakan coba lagi nanti (biasanya reset setiap 24 jam) atau upgrade ke paid plan untuk quota lebih besar.", 2)
			return
		}
		sendMessageWithRetry(context.Background(), v.Info.Chat, "❌ *Maaf,* terjadi kesalahan saat membuat gambar. Silakan coba lagi nanti atau gunakan deskripsi yang lebih sederhana.", 2)
		return
	}

	// Create caption for the image
	caption := fmt.Sprintf("🎨 *Gambar AI Generated*\n\nPrompt: %s\n\nDibuat menggunakan Gemini 2.0 Flash Preview Image Generation", prompt)

	// Send the generated image
	err = sendImageWithRetry(context.Background(), v.Info.Chat, imageBase64, caption, 3)
	if err != nil {
		log.Printf("Failed to send generated image: %v", err)

		// Check if it was sent as data URL, thumbnail, or fallback message
		if strings.Contains(err.Error(), "data URL") || strings.Contains(err.Error(), "fallback message") || strings.Contains(err.Error(), "thumbnail") {
			log.Printf("Image sent successfully (as data URL, thumbnail, or fallback)")
			return
		}

		// Send fallback message with instructions
		fallbackMessage := fmt.Sprintf("🎨 *Gambar Berhasil Dibuat*\n\nPrompt: %s\n\n❌ *Gagal Mengirim Gambar*\n\nGambar berhasil dibuat oleh AI tetapi gagal dikirim ke WhatsApp. Kemungkinan penyebab:\n• Ukuran file terlalu besar\n• Masalah koneksi\n• Format tidak didukung\n\nSilakan coba lagi dengan deskripsi yang lebih sederhana atau tunggu beberapa saat.", prompt)
		sendMessageWithRetry(context.Background(), v.Info.Chat, fallbackMessage, 2)
		return
	}

	log.Printf("Successfully generated and sent image for prompt: %s", prompt)
}
