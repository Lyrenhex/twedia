package veadotube

import (
	"bytes"
	"encoding/json"
	"strings"
)

type stateEventResponseList struct {
	Event   string            `json:"event"`
	Type    string            `json:"type"`
	Id      string            `json:"id"`
	Name    string            `json:"name"`
	Payload payloadStatesList `json:"payload"`
}

type payloadStatesList struct {
	Event  string          `json:"event"`
	States []responseState `json:"states"`
}

type responseState struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type stateEventResponsePeek struct {
	Event   string           `json:"event"`
	Type    string           `json:"type"`
	Id      string           `json:"id"`
	Name    string           `json:"name"`
	Payload payloadStatePeek `json:"payload"`
}

type payloadStatePeek struct {
	Event string `json:"event"`
	State string `json:"state"`
}

type instanceEventResponseInfo struct {
	Event    string `json:"event"`
	Name     string `json:"name"`
	Id       string `json:"id"`
	Version  string `json:"version"`
	Language string `json:"language"`
	Server   string `json:"server"`
}

// Listen for messages from the Veadotube WebSocket connection and
// handle internal state changes appropriately.
func (v *Veadotube) listen() {
	respList := &stateEventResponseList{}
	respPeek := &stateEventResponsePeek{}
	respInfo := &instanceEventResponseInfo{}
	for {
		_, msg, err := v.Connection.ReadMessage()
		if err != nil {
			l.Println("WebSocket read:", err)
		}
		channel, s, _ := strings.Cut(string(msg), ":")
		msg = bytes.Trim([]byte(s), "\x00")
		switch channel {
		case "nodes": // https://veado.tube/help/docs/websocket/#nodes
			err = json.Unmarshal(msg, respList)
			if err == nil {
				v.handleResponseStateList(respList)
				continue
			}
			err = json.Unmarshal(msg, respPeek)
			if err == nil {
				v.handleResponseStatePeek(respPeek)
				continue
			}
		case "instance": // https://veado.tube/help/docs/websocket/#instance
			err = json.Unmarshal(msg, respInfo)
			if err == nil {
				// stub:
				// we don't currently request nor make use of the `info` event
				// but handling it is a little cleaner than a benign log msg.
				continue
			}
		default:
		}
		// we've fallen through - unhandled event!
		l.Printf("Unhandled message from channel '%s': %s\n", channel, s)
	}
}

func (v *Veadotube) handleResponseStateList(resp *stateEventResponseList) {
	for _, state := range resp.Payload.States {
		v.StateMap[state.Name] = state.Id
	}
}

func (v *Veadotube) handleResponseStatePeek(_ *stateEventResponsePeek) {
	l.Fatal("TODO: handle state peek response")
}
