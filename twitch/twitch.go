package twitch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
)

const pubSubAPI string = "wss://pubsub-edge.twitch.tv"

type apiResp struct {
	Users   []user `json:"data"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}
type user struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}
type rewardImg struct {
	URL1x string `json:"url_1x"`
	URL2x string `json:"url_2x"`
	URL4x string `json:"url_4x"`
}
type rewardMax struct {
	IsEnabled    bool `json:"is_enabled"`
	MaxPerStream int  `json:"max_per_stream"`
}
type reward struct {
	ID                                string    `json:"id"`
	ChannelID                         string    `json:"channel_id"`
	Title                             string    `json:"title"`
	Prompt                            string    `json:"prompt"`
	Cost                              int       `json:"cost"`
	IsUserInputRequired               bool      `json:"is_user_input_required"`
	IsSubOnly                         bool      `json:"is_sub_only"`
	Image                             rewardImg `json:"image"`
	DefaultImage                      rewardImg `json:"default_image"`
	BackgroundColor                   string    `json:"background_color"`
	IsEnabled                         bool      `json:"is_enabled"`
	IsPaused                          bool      `json:"is_paused"`
	IsInStock                         bool      `json:"is_in_stock"`
	MaxPerStream                      rewardMax `json:"max_per_stream"`
	ShouldRedemptionsSkipRequestQueue bool      `json:"should_redemptions_skip_request_queue"`
}

// Redemption represents a Channel Point reward redemption on Twitch.
type Redemption struct {
	ID         string    `json:"id"`
	User       user      `json:"user"`
	ChannelID  string    `json:"channel_id"`
	RedeemedAt time.Time `json:"redeemed_at"`
	Reward     reward    `json:"reward"`
	UserInput  string    `json:"user_input"`
	Status     string    `json:"status"`
}
type msgData struct {
	Timestamp  time.Time  `json:"timestamp"`
	Redemption Redemption `json:"redemption"`
}
type message struct {
	Type string  `json:"type"`
	Data msgData `json:"data"`
}
type data struct {
	Topics    []string `json:"topics"`
	AuthToken string   `json:"auth_token"`
	Topic     string   `json:"topic"`
	Message   string   `json:"message"`
}
type pubSub struct {
	Type  string `json:"type"`
	Nonce string `json:"nonce"`
	Data  data   `json:"data"`
	Error string `json:"error"`
}

// GetOAuthToken gets a User OAuth Token from the Twitch API and returns it as a string.
// This function needs further work: it is not fully automated, requiring user involvement (which also has an ugly UX)
func GetOAuthToken(clientID string) string {
	browser.OpenURL("https://id.twitch.tv/oauth2/authorize?client_id=" + clientID + "&redirect_uri=http://localhost&response_type=token&scope=channel_read%20channel:read:redemptions")

	ctx, cancel := context.WithCancel(context.Background())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Please see token in the address bar and copy to Twedia.")
		cancel()
	})
	srv := &http.Server{Addr: ":80"}
	go srv.ListenAndServe()
	<-ctx.Done()
	if err := srv.Shutdown(ctx); err != nil && err != context.Canceled {
		log.Println(err)
	}

	fmt.Print("Please enter OAuth token: ")

	var token string
	var err error
	for {
		reader := bufio.NewReader(os.Stdin)
		token, err = reader.ReadString('\n')
		if err == nil {
			break
		}
	}
	token = strings.Replace(strings.Replace(token, "\n", "", -1), "\r", "", -1)

	return token
}

// GetChannelID retrieves the channel ID for the OAuth token provided, and returns it as a string
func GetChannelID(token, clientID string) (string, error) {
	chanInfo := &apiResp{}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Client-Id", clientID)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, chanInfo)

	if chanInfo.Status != 0 {
		// Something went wrong...
		// Also love that the Twitch API reference doesn't seem to explain what it does if something goes wrong?
		return "", errors.New(strings.ToLower(chanInfo.Message))
	}

	if len(chanInfo.Users) == 0 {
		// ... we presumably had a successful response, but no users were returned. this... shouldn't happen?
		return "", errors.New("no users returned")
	}

	return chanInfo.Users[0].ID, nil
}

// ListenChannelPoints starts a WebSocket listening to the Twitch PubSub API for Channel Point redemptions, which calls callback with the provided file handle and the reward title as a string
func ListenChannelPoints(chanID, clientID, oauthToken string, callback func(Redemption)) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	attempts := 0
	for {
		attempts++
		c, _, err := websocket.DefaultDialer.Dial(pubSubAPI, nil)
		if err != nil {
			time.Sleep(time.Second + (time.Duration(rand.Intn(1000))*time.Millisecond)*time.Duration(attempts))
			continue
		}

		listenReq := pubSub{
			Type: "LISTEN",
			Data: data{
				Topics: []string{
					"channel-points-channel-v1." + chanID,
				},
				AuthToken: oauthToken,
			},
		}
		listenReqJSON, _ := json.Marshal(listenReq)
		c.WriteMessage(websocket.TextMessage, listenReqJSON)

		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					err := c.WriteMessage(websocket.TextMessage, []byte("{\"type\": \"PING\"}"))
					if err != nil {
						log.Println("PubSub write: ", err)
					}
				case <-interrupt:
					err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("PubSub write close: ", err)
						return
					}
					time.Sleep(time.Second)
					return
				}
			}
		}()

		for {
			_, msg, err := c.ReadMessage()
			resp := &pubSub{}
			json.Unmarshal(msg, resp)
			if err != nil {
				log.Println("PubSub read: ", err)
				continue
			}
			switch resp.Type {
			case "RESPONSE":
				if resp.Error == "ERR_BADAUTH" {
					log.Println("Bad PubSub auth, requesting new token.")
					GetOAuthToken(clientID)
					// this attempt was a failure; interrupt the keepalive and try again
					interrupt <- os.Interrupt
					go ListenChannelPoints(chanID, clientID, oauthToken, callback)
				} else if resp.Error != "" {
					log.Println("PubSub API error: ", resp.Error)
				}
			case "MESSAGE":
				message := &message{}
				json.Unmarshal([]byte(resp.Data.Message), message)
				if message.Type == "reward-redeemed" {
					callback(message.Data.Redemption)
				}
			}
		}
	}
}
