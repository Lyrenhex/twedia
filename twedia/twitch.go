package twedia

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

const twitchPubSubAPI string = "wss://pubsub-edge.twitch.tv"

type twitchAPI5Resp struct {
	ID string `json:"_id"`
}
type twitchReward struct {
	Title string `json:"title"`
}
type twitchData struct {
	Topics    []string     `json:"topics"`
	AuthToken string       `json:"auth_token"`
	Reward    twitchReward `json:"reward"`
}
type twitchPubSub struct {
	Type  string     `json:"type"`
	Nonce string     `json:"nonce"`
	Data  twitchData `json:"data"`
	Error string     `json:"error"`
}

// GetChannelID retrieves the channel ID for the OAuth token defined in the "TWITCH_PUBSUB_OAUTH_TOKEN" environment variable, and returns it as a string
func GetChannelID() string {
	chanInfo := &twitchAPI5Resp{}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/kraken/channel", nil)
	req.Header.Add("Accept", "application/vnd.twitchtv.v5+json")
	req.Header.Add("Authorization", "OAuth "+os.Getenv("TWITCH_PUBSUB_OAUTH_TOKEN"))
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
				AuthToken: os.Getenv("TWITCH_OAUTH_TOKEN"),
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
				if resp.Error != "" {
					log.Println("PubSub API error: ", resp.Error)
				}
			case "reward-redeemed":
				callback(resp.Data.Reward.Title, f)
			}
		}
	}
}
