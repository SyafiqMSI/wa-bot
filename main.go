package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/glebarez/sqlite"
	"github.com/joho/godotenv"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	"whatsmeow-api/handler"
)

func main() {
	if loadErr := godotenv.Load(); loadErr != nil {
		log.Printf("No .env file found or failed to load: %v", loadErr)
	}

	ctx := context.Background()

	logger := waLog.Stdout("whatsapp", "INFO", true)

	// Initialize memory store
	memoryPath := os.Getenv("MEMORY_FILE")
	if memoryPath == "" {
		memoryPath = "memory.json"
	}
	if err := handler.InitMemory(memoryPath); err != nil {
		log.Printf("Failed to initialize memory store: %v", err)
	}

	// Ensure session directory exists
	if err := os.MkdirAll("session", 0755); err != nil {
		log.Fatalf("Failed to create session directory: %v", err)
	}

	container, err := sqlstore.New(ctx, "sqlite", "file:session/store.db?_pragma=foreign_keys(1)", logger)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Fatalf("Failed to get device: %v", err)
	}

	handler.WaClient = whatsmeow.NewClient(deviceStore, logger)
	handler.WaClient.AddEventHandler(handler.EventHandler)

	if handler.WaClient.Store.ID == nil {
		qrChan, _ := handler.WaClient.GetQRChannel(ctx)
		err = handler.WaClient.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("QR Code:")
				fmt.Println(evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = handler.WaClient.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
	}

	r := handler.SetupRoutes()
	httpHandler := handler.SetupCORS(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("🚀 WhatsApp Bot Server starting...")
	log.Printf("🌐 Port: %s", port)
	log.Printf("🔗 WhatsApp Connected: %t", handler.WaClient.IsConnected())
	log.Printf("📋 Available endpoints:")
	log.Printf("   GET  / - Status")
	log.Printf("   GET  /health - Health check")
	log.Printf("   GET  /groups - Get joined groups")
	log.Printf("   GET  /idx - Get IDX market data")
	log.Printf("   POST /send-message - Send message")
	log.Printf("   POST /send-bulk-same-message - Bulk same message")
	log.Printf("   POST /send-bulk-different-messages - Bulk different messages")
	log.Printf("   POST /github-webhook - GitHub webhook (supports ?jid=<target_jid>)")
	log.Printf("✅ Server is ready and listening on port %s", port)

	log.Fatal(http.ListenAndServe(":"+port, httpHandler))
}
