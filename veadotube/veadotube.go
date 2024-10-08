package veadotube

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type instanceData struct {
	Time   int64  `json:"time"`
	Name   string `json:"name"`
	Server string `json:"server"`
}

type stateEventRequestBasic struct {
	Event   string            `json:"event"`
	Type    string            `json:"type"`
	Id      string            `json:"id"`
	Payload payloadBasicEvent `json:"payload"`
}

type stateEventRequestWithState struct {
	Event   string                `json:"event"`
	Type    string                `json:"type"`
	Id      string                `json:"id"`
	Payload payloadEventWithState `json:"payload"`
}

func newStateEventRequestBasic(payload payloadBasicEvent) string {
	r, err := json.Marshal(stateEventRequestBasic{
		Event:   "payload",
		Type:    "stateEvents",
		Id:      "mini",
		Payload: payload,
	})
	if err != nil {
		log.Println("Marshaling state event request: ", err)
	}
	return "nodes:" + string(r)
}

func newStateEventRequestWithState(payload payloadEventWithState) string {
	r, err := json.Marshal(stateEventRequestWithState{
		Event:   "payload",
		Type:    "stateEvents",
		Id:      "mini",
		Payload: payload,
	})
	if err != nil {
		log.Println("Marshaling state event request: ", err)
	}
	return "nodes:" + string(r)
}

type payloadBasicEvent struct {
	Event string `json:"event"`
}

type payloadEventWithState struct {
	Event string `json:"event"`
	State string `json:"state"`
}

type stateEventResponseList struct {
	Event   string            `json:"event"`
	Type    string            `json:"type"`
	Id      string            `json:"id"`
	Name    string            `json:"name"`
	Payload payloadStatesList `json:"payload"`
}

type stateEventResponsePeek struct {
	Event   string           `json:"event"`
	Type    string           `json:"type"`
	Id      string           `json:"id"`
	Name    string           `json:"name"`
	Payload payloadStatePeek `json:"payload"`
}

type payloadStatesList struct {
	Event  string          `json:"event"`
	States []responseState `json:"states"`
}

type payloadStatePeek struct {
	Event string `json:"event"`
	State string `json:"state"`
}

type responseState struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

var currentInstance instanceData
var connection *websocket.Conn
var stateMap map[string]string = make(map[string]string)

func Connect() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Println("Error retrieving home directory: ", err)
		return
	}
	s := string(os.PathSeparator)
	instanceDir := home + s + ".veadotube" + s + "instances"
	instanceFiles, err := os.ReadDir(instanceDir)
	if err != nil {
		log.Println("Error retrieving veadotube instance list: ", err)
		return
	}
	var instances []instanceData
	for _, f := range instanceFiles {
		if strings.HasPrefix(f.Name(), "mini-") {
			i := &instanceData{}
			data, err := os.ReadFile(instanceDir + s + f.Name())
			if err != nil {
				log.Println("Error reading instance data for "+f.Name()+": ", err)
				continue
			}
			json.Unmarshal(data, i)
			if (time.Now().Unix() - i.Time) > 10 {
				log.Println("Skipping stale instance ", f.Name())
				continue
			}
			if i.Server == "" || i.Server == ":0" {
				log.Println("Skipping instance with no websocket server ", f.Name())
				continue
			}
			instances = append(instances, *i)
		}
	}

	if len(instances) == 1 {
		currentInstance = instances[0]
	} else if len(instances) == 0 {
		log.Println("No valid instances found.")
		return
	} else {
		fmt.Println("Found the following running veadotube mini instances:")
		for i, instance := range instances {
			fmt.Println("\t", i+1, ": ", instance.Name, " (", instance.Server, ")")
		}
		for {
			fmt.Print("Select an instance (leave blank for none): ")
			reader := bufio.NewReader(os.Stdin)
			opt, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			opt = strings.ToLower(strings.Replace(strings.Replace(opt, "\n", "", -1), "\r", "", -1))
			if opt == "" {
				return
			}
			iopt, err := strconv.Atoi(opt)
			if err != nil {
				continue
			}
			iopt -= 1
			if iopt < 0 || iopt >= len(instances) {
				fmt.Println("Please enter a valid instance number.")
				continue
			}
			currentInstance = instances[iopt]
			break
		}
	}
	log.Println("Connecting to instance ", currentInstance.Name, "(", currentInstance.Server, ")...")
	log.Println("Connected.")
	connection, _, err = websocket.DefaultDialer.Dial("ws://"+currentInstance.Server+"?n=twedia", nil)
	if err != nil {
		log.Println("Error connecting to veadotube instance: ", err)
		return
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-interrupt:
				err := connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Println("PubSub write close: ", err)
					return
				}
				time.Sleep(time.Second)
				return
			}
		}
	}()

	err = connection.WriteMessage(websocket.TextMessage, []byte(newStateEventRequestBasic(payloadBasicEvent{
		Event: "list",
	})))
	if err != nil {
		log.Println("Error requesting states list: ", err)
		return
	}
	_, msg, err := connection.ReadMessage()
	if err != nil {
		log.Println("WebSocket read: ", err)
	}
	s = string(msg)[6:]
	msg = bytes.Trim([]byte(s), "\x00")
	resp := &stateEventResponseList{}
	err = json.Unmarshal(msg, resp)
	if err != nil {
		log.Println("Response unmarshal: ", err)
	}
	for _, state := range resp.Payload.States {
		stateMap[state.Name] = state.Id
	}
}

func SetState(state string) {
	if connection == nil {
		log.Println("Failed to set veadotube state to '" + state + "': no connection")
		return
	}
	if stateMap[state] == "" {
		log.Println("Unknown state '" + state + "'.")
		return
	}
	connection.WriteMessage(websocket.TextMessage, []byte(newStateEventRequestWithState(payloadEventWithState{
		Event: "set",
		State: stateMap[state],
	})))
}
