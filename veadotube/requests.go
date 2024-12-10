package veadotube

import "encoding/json"

type stateEventRequestBasic struct {
	Event   string            `json:"event"`
	Type    string            `json:"type"`
	Id      string            `json:"id"`
	Payload payloadBasicEvent `json:"payload"`
}

type payloadBasicEvent struct {
	Event string `json:"event"`
}

type stateEventRequestWithState struct {
	Event   string                `json:"event"`
	Type    string                `json:"type"`
	Id      string                `json:"id"`
	Payload payloadEventWithState `json:"payload"`
}

type payloadEventWithState struct {
	Event string `json:"event"`
	State string `json:"state"`
}

func newStateEventRequestBasic(payload payloadBasicEvent) string {
	r, err := json.Marshal(stateEventRequestBasic{
		Event:   "payload",
		Type:    "stateEvents",
		Id:      "mini",
		Payload: payload,
	})
	if err != nil {
		l.Println("Marshaling state event request: ", err)
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
		l.Println("Marshaling state event request: ", err)
	}
	return "nodes:" + string(r)
}
