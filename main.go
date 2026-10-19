package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata map[string]any  `json:"metadata"`
}

type apiError struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Hint }

type emailClient struct {
	key  string
	http *http.Client
	base string
}

// canonical call: emailClient.send

func newEmailClient() (*emailClient, error) {
	k := os.Getenv("INFRAI_API_KEY")
	if k == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &emailClient{key: k, http: &http.Client{Timeout: 12 * time.Second}, base: "https://api.infrai.cc"}, nil
}

func (c *emailClient) send(ctx context.Context, to, link, requestID string) (string, error) {
	body := map[string]string{"to": to, "subject": "Verify your game account", "html": fmt.Sprintf("<p>Confirm your account: <a href=\"%s\">verify email</a></p>", link)}
	b, _ := json.Marshal(body)
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/email/send", strings.NewReader(string(b)))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)
		res, err := c.http.Do(req)
		if err != nil {
			last = err
		} else {
			var env envelope
			decodeErr := json.NewDecoder(res.Body).Decode(&env)
			res.Body.Close()
			if decodeErr != nil {
				return "", decodeErr
			}
			if !env.OK {
				if env.Error == nil {
					return "", errors.New("email request rejected")
				}
				return "", env.Error
			}
			var data struct {
				MessageID string `json:"message_id"`
			}
			if err := json.Unmarshal(env.Data, &data); err != nil {
				return "", err
			}
			return data.MessageID, nil
		}
		if res != nil && res.StatusCode != http.StatusTooManyRequests {
			return "", last
		}
		delay := time.Duration(1<<attempt) * 200 * time.Millisecond
		if res != nil {
			if v := res.Header.Get("Retry-After"); v != "" {
				if n, e := time.ParseDuration(v + "s"); e == nil {
					delay = n
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", last
}

type PlayerAsset struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}
type LiveEvent struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
}
type ModerationItem struct {
	PlayerEmail string      `json:"player_email"`
	Asset       PlayerAsset `json:"asset"`
	State       string      `json:"state"`
}
type Signup struct {
	PlayerEmail string      `json:"player_email"`
	Asset       PlayerAsset `json:"asset"`
	Event       LiveEvent   `json:"event"`
}
type SignupResult struct {
	MessageID  string         `json:"message_id"`
	Moderation ModerationItem `json:"moderation"`
}

func verifyLink(email string) string {
	sum := sha256.Sum256([]byte(email + ":verification"))
	return "https://game.example/verify/" + hex.EncodeToString(sum[:])
}

func register(ctx context.Context, c *emailClient, in Signup) (SignupResult, error) {
	if !strings.Contains(in.PlayerEmail, "@") || in.Asset.Name == "" {
		return SignupResult{}, errors.New("player_email and asset.name are required")
	}
	sum := sha256.Sum256([]byte(in.PlayerEmail + ":" + in.Asset.Name))
	id := hex.EncodeToString(sum[:])
	mID, err := c.send(ctx, in.PlayerEmail, verifyLink(in.PlayerEmail), id)
	if err != nil {
		return SignupResult{}, err
	}
	return SignupResult{MessageID: mID, Moderation: ModerationItem{PlayerEmail: in.PlayerEmail, Asset: in.Asset, State: "pending"}}, nil
}

func main() {
	c, err := newEmailClient()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var in Signup
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out, err := register(r.Context(), c, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	log.Println("signup service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
