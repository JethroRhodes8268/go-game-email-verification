package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterSendsVerificationAndQueuesAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/email/send" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") == "" {
			t.Fatal("missing request headers")
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["to"] != "player@example.com" || body["html"] == "" {
			t.Fatal("verification payload missing")
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"message_id": "msg-1"}})
	}))
	defer server.Close()
	c := &emailClient{key: "test-key", http: server.Client(), base: server.URL}
	out, err := register(context.Background(), c, Signup{PlayerEmail: "player@example.com", Asset: PlayerAsset{Name: "neon-sword", Kind: "skin"}, Event: LiveEvent{Name: "launch"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.MessageID != "msg-1" || out.Moderation.State != "pending" {
		t.Fatalf("unexpected result: %+v", out)
	}
}
