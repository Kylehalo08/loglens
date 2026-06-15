package main

import (
	"context"
	"log"
	"os"

	"github.com/Kylehalo08/loglens/sdk/go/loglens"
)

func main() {
	apiKey := os.Getenv("LOGLENS_API_KEY")
	if apiKey == "" {
		log.Fatal("LOGLENS_API_KEY is required")
	}

	client := loglens.NewClient(apiKey)
	ctx := context.Background()

	if err := client.Error(ctx, "payment failed", map[string]any{
		"order_id": "123",
		"amount":   499,
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("log sent")
}
