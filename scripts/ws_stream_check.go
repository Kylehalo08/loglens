//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	orgID := os.Getenv("ORG_ID")
	svcID := os.Getenv("SVC_ID")
	token := os.Getenv("TOKEN")
	apiKey := os.Getenv("API_KEY")
	if orgID == "" || svcID == "" || token == "" || apiKey == "" {
		fmt.Println("set ORG_ID, SVC_ID, TOKEN, API_KEY")
		os.Exit(1)
	}

	url := fmt.Sprintf("ws://localhost:8080/orgs/%s/services/%s/logs/stream", orgID, svcID)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			fmt.Printf("dial failed: %v (http %d)\n", err, resp.StatusCode)
		} else {
			fmt.Printf("dial failed: %v\n", err)
		}
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("WebSocket connected, waiting for log...")

	done := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		done <- msg
	}()

	time.Sleep(500 * time.Millisecond)

	body := strings.NewReader(`{"severity":"INFO","message":"websocket live stream test","metadata":{"via":"ws-test"}}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8081/v1/logs", body)
	if err != nil {
		fmt.Printf("ingest request failed: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	ingestResp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("ingest failed: %v\n", err)
		os.Exit(1)
	}
	ingestResp.Body.Close()
	fmt.Printf("Ingest status: %d\n", ingestResp.StatusCode)

	select {
	case err := <-errCh:
		fmt.Printf("read failed: %v\n", err)
		os.Exit(1)
	case msg := <-done:
		fmt.Printf("Received on WebSocket: %s\n", string(msg))
		if !strings.Contains(string(msg), "websocket live stream test") {
			fmt.Println("FAIL: expected message not in payload")
			os.Exit(1)
		}
		fmt.Println("PASS: live stream delivered log within timeout")
	}
}
