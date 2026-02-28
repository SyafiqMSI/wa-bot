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

	"whatsmeow-api/services/gemini"
	"whatsmeow-api/whatsapp"
)

func main() {
	if loadErr := godotenv.Load(); loadErr != nil {
		log.Printf("No .env file found or failed to load: %v", loadErr)
	}

	ctx := context.Background()

	logger := waLog.Stdout("whatsapp", "INFO", true)

	memoryPath := os.Getenv("MEMORY_FILE")
	if memoryPath == "" {
		memoryPath = "memory.json"
	}
	if err := gemini.InitMemory(memoryPath); err != nil {
		log.Printf("Failed to initialize memory store: %v", err)
	}

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

	whatsapp.Client = whatsmeow.NewClient(deviceStore, logger)
	whatsapp.Client.AddEventHandler(handler.EventHandler)

	if whatsapp.Client.Store.ID == nil {
		qrChan, _ := whatsapp.Client.GetQRChannel(ctx)
		err = whatsapp.Client.Connect()
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
		err = whatsapp.Client.Connect()
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

	log.Printf("[server] WhatsApp Bot Server starting...")
	log.Printf("[server] Port: %s", port)
	log.Printf("[server] WhatsApp Connected: %t", whatsapp.Client.IsConnected())
	log.Printf("[server] Server is ready and listening on port %s", port)

	log.Fatal(http.ListenAndServe(":"+port, httpHandler))
}
