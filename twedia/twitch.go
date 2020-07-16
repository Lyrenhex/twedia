package twedia

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

const twitchPubSubAPI string = "wss://pubsub-edge.twitch.tv"

type twitchAPI5Resp struct {
	ID string `json:"_id"`
}
type twitchReward struct {
	Title string `json:"title"`
}
type twitchRedemption struct {
	Reward twitchReward `json:"reward"`
}
type twitchMsgData struct {
	Redemption twitchRedemption `json:"redemption"`
}
type twitchMessage struct {
	Type string        `json:"type"`
	Data twitchMsgData `json:"data"`
}
type twitchData struct {
	Topics    []string `json:"topics"`
	AuthToken string   `json:"auth_token"`
	Topic     string   `json:"topic"`
	Message   string   `json:"message"`
}
type twitchPubSub struct {
	Type  string     `json:"type"`
	Nonce string     `json:"nonce"`
	Data  twitchData `json:"data"`
	Error string     `json:"error"`
}

func GetOAuthToken() string {
	token, set := os.LookupEnv("TWITCH_PUBSUB_OAUTH_TOKEN")
	if set {
		return token
	}

	browser.OpenURL("https://id.twitch.tv/oauth2/authorize?client_id=" + os.Getenv("TWITCH_CLIENT_ID") + "&redirect_uri=http://localhost&response_type=token&scope=channel_read%20channel:read:redemptions")

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

	var err error
	for {
		reader := bufio.NewReader(os.Stdin)
		token, err = reader.ReadString('\n')
		if err == nil {
			break
		}
	}
	token = strings.Replace(strings.Replace(token, "\n", "", -1), "\r", "", -1)

	os.Setenv("TWITCH_PUBSUB_OAUTH_TOKEN", token)

	return token
}

// GetChannelID retrieves the channel ID for the OAuth token provided, and returns it as a string
func GetChannelID(token string) string {
	chanInfo := &twitchAPI5Resp{}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/kraken/channel", nil)
	req.Header.Add("Accept", "application/vnd.twitchtv.v5+json")
	req.Header.Add("Authorization", "OAuth "+token)
	req.Header.Add("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, chanInfo)

	return chanInfo.ID
}

// ListenChannelPoints starts a WebSocket listening to the Twitch PubSub API for Channel Point redemptions, which calls callback with the provided file handle and the reward title as a string
func ListenChannelPoints(cID string, f *os.File, callback func(string, *os.File)) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	attempts := 0
	for {
		attempts++
		c, _, err := websocket.DefaultDialer.Dial(twitchPubSubAPI, nil)
		if err != nil {
			time.Sleep(time.Second + (time.Duration(rand.Intn(1000))*time.Millisecond)*time.Duration(attempts))
			continue
		}
		defer c.Close()

		listenReq := twitchPubSub{
			Type: "LISTEN",
			Data: twitchData{
				Topics: []string{
					"channel-points-channel-v1." + cID,
				},
				AuthToken: os.Getenv("TWITCH_PUBSUB_OAUTH_TOKEN"),
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
					select {
					case <-time.After(time.Second):
					}
					return
				}
			}
		}()

		for {
			_, msg, err := c.ReadMessage()
			resp := &twitchPubSub{}
			json.Unmarshal(msg, resp)
			if err != nil {
				log.Println("PubSub read: ", err)
				continue
			}
			switch resp.Type {
			case "RESPONSE":
				if resp.Error == "ERR_BADAUTH" {
					GetOAuthToken()
				} else if resp.Error != "" {
					log.Println("PubSub API error: ", resp.Error)
				}
			case "MESSAGE":
				message := &twitchMessage{}
				json.Unmarshal([]byte(resp.Data.Message), message)
				if message.Type == "reward-redeemed" {
					callback(message.Data.Redemption.Reward.Title, f)
				}
			}
		}
	}
}
