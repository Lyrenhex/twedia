package veadotube

import (
	"bufio"
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

type InstanceData struct {
	Time   int64  `json:"time"`
	Name   string `json:"name"`
	Server string `json:"server"`
}

type Veadotube struct {
	CurrentInstance InstanceData
	Connection      *websocket.Conn
	StateMap        map[string]string
}

var l *log.Logger = log.New(os.Stdout, "[veadotube] ", log.LstdFlags|log.Lshortfile|log.Lmsgprefix)

// Select a running Veadotube instance from the user's home directory.
//
// This function involves direct user input, and may return `nil` on an error
// or if multiple instances are available to be selected but the user selects
// none of them.
func New() (*Veadotube, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		l.Println("Error retrieving home directory:", err)
		return nil, err
	}
	s := string(os.PathSeparator)
	instanceDir := home + s + ".veadotube" + s + "instances"
	instanceFiles, err := os.ReadDir(instanceDir)
	if err != nil {
		l.Println("Error retrieving veadotube instance list:", err)
		return nil, err
	}
	var instances []InstanceData
	for _, f := range instanceFiles {
		if strings.HasPrefix(f.Name(), "mini-") {
			i := &InstanceData{}
			data, err := os.ReadFile(instanceDir + s + f.Name())
			if err != nil {
				l.Printf("Error reading instance data for %s: %s\n", f.Name(), err)
				continue
			}
			json.Unmarshal(data, i)
			if (time.Now().Unix() - i.Time) > 10 {
				l.Println("Skipping stale instance", f.Name())
				continue
			}
			if i.Server == "" || i.Server == ":0" {
				l.Println("Skipping instance with no websocket server", f.Name())
				continue
			}
			instances = append(instances, *i)
		}
	}

	v := Veadotube{}

	if len(instances) == 1 {
		v.CurrentInstance = instances[0]
	} else if len(instances) == 0 {
		l.Println("No valid instances found.")
		return nil, nil
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
				return nil, nil
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
			v.CurrentInstance = instances[iopt]
			break
		}
	}

	v.StateMap = make(map[string]string)

	return &v, nil
}

// Connect to the Veadotube instance.
func (v *Veadotube) Connect() {
	l.Printf("Connecting to instance %s (%s)...\n", v.CurrentInstance.Name, v.CurrentInstance.Server)
	var err error
	v.Connection, _, err = websocket.DefaultDialer.Dial("ws://"+v.CurrentInstance.Server+"?n=twedia", nil)
	if err != nil {
		l.Println("Error connecting to veadotube instance:", err)
		return
	}
	l.Println("Connected.")
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-interrupt:
				err := v.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					l.Println("PubSub write close:", err)
					return
				}
				time.Sleep(time.Second)
				return
			}
		}
	}()

	go v.listen()

	err = v.Connection.WriteMessage(websocket.TextMessage, []byte(newStateEventRequestBasic(payloadBasicEvent{
		Event: "list",
	})))
	if err != nil {
		l.Println("Error requesting states list:", err)
		return
	}
}

// Set the active Veadotube state to that with the provided state name.
func (v *Veadotube) SetState(state string) {
	if v.Connection == nil {
		l.Printf("Failed to set veadotube state to '%s': no connection\n", state)
		return
	}
	if v.StateMap[state] == "" {
		l.Printf("Unknown state '%s'.\n", state)
		return
	}
	v.Connection.WriteMessage(websocket.TextMessage, []byte(newStateEventRequestWithState(payloadEventWithState{
		Event: "set",
		State: v.StateMap[state],
	})))
}
