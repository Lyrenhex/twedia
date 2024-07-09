package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc"
	"github.com/lyrenhex/twedia/twedia"
)

type Config struct {
	Username           string    `json:"username"`
	Channel            string    `json:"channel"`
	ClientID           string    `json:"clientID"`
	ClientSecret       string    `json:"clientSecret"`
	MusicDir           string    `json:"musicDir"`
	MusicFile          string    `json:"musicFile"`
	OauthToken         string    `json:"oauthToken"`
	PubsubOauthToken   string    `json:"pubsubOauthToken"`
	MusicCollectionURL string    `json:"musicCollectionURL"`
	ChatCommands       []command `json:"chatCommands"`
	PointRewards       []reward  `json:"pointRewards"`
}

type command struct {
	Trigger string `json:"trigger"`
	Action  action `json:"action"`
}

type reward struct {
	Title  string `json:"rewardTitle"`
	Action action `json:"action"`
}

type action struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Song   string `json:"title"`
}

var config Config
var artists twedia.Music
var t *twitch.Client
var channelID string
var musicPlayer twedia.Player
var speechPlayer twedia.Player

var lastSpeech time.Time = time.Unix(0, 0)

func init() {
	var err error
	config, err = loadConfig(os.Getenv("TWITCH_CONFIG_FILE"))
	if err != nil {
		panic(err)
	}

	if !exists("tts") {
		os.Mkdir("tts", 0644)
	}

	err = twedia.GetSongs(&artists, config.MusicCollectionURL)
	if err != nil {
		log.Fatal(err)
	}

	err = twedia.InitSpeaker()
	if err != nil {
		log.Println("Error initialising speaker:", err)
	}

	musicPlayer = twedia.NewPlayer()
	speechPlayer = twedia.NewPlayer()

	for {
		channelID, err = twedia.GetChannelID(config.PubsubOauthToken, config.ClientID)
		if err == nil {
			break
		} else if err.Error() == "invalid oauth token" {
			config.PubsubOauthToken = twedia.GetOAuthToken(config.ClientID)
			config.saveConfig(os.Getenv("TWITCH_CONFIG_FILE"))
		} else {
			log.Println("Error obtaining channel ID:", err)
			os.Exit(1)
		}
	}

	// Seed the random Source such that we don't always listen to Blessed are the Teamakers...
	rand.Seed(time.Now().UnixNano())

	fmt.Println(`Twedia Music Manager
	
Commands:
	start : start playing random music
	pause : pause / unpause the current song
	skip  : skip the current song
	stop  : stop playing music
	select: play a specific song
	quit  : exit program`)
}

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "%x", bs)
	return buf.String()
}

func loadConfig(s string) (Config, error) {
	var config Config

	f, err := os.Open(s)
	if err != nil {
		return config, err
	}
	defer f.Close()

	b, _ := io.ReadAll(f)

	json.Unmarshal(b, &config)

	return config, nil
}

func (c *Config) saveConfig(s string) error {
	f, err := os.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()

	json, err := json.Marshal(c)
	if err != nil {
		return err
	}

	err = os.WriteFile(s, json, 0777)
	if err != nil {
		return err
	}

	return nil
}

func exists(fp string) bool {
	// cheers to https://stackoverflow.com/a/12518877/4897375 (CC-BY-SA 4.0)
	if _, err := os.Stat(fp); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		return false
	} else {
		fmt.Printf("Unexpected error when verifying file existence: %s\n", err)
		return false
	}
}

func play(artist twedia.Artist, album twedia.Album, song twedia.Song) error {
	var f *os.File
	var err error
	for {
		f, err = os.Create(config.MusicFile)
		if err == nil {
			break
		}
	}
	defer f.Close()

	// open the song for playing
	s := string(os.PathSeparator)

	path := config.MusicDir + s + artist.Artist + s + album.Name + s + song.Title

	if exists(path + ".mp3") {
		path += ".mp3"
	} else if exists(path + ".wav") {
		path += ".wav"
	} else if exists(path + ".ogg") {
		path += ".ogg"
	} else if exists(path + ".flac") {
		path += ".flac"
	} else {
		log.Println("Song file cannot be found: " + path)
		return errors.New("Song file cannot be found: " + path)
	}

	f.WriteString(fmt.Sprintf("\n%s, by %s", song.Title, artist.Artist))
	if song.URL != "" {
		t.Say(config.Channel, fmt.Sprintf("Playing %s by %s. Listen on YouTube: %s", song.Title, artist.Artist, song.URL))
	} else {
		t.Say(config.Channel, fmt.Sprintf("Playing %s by %s.", song.Title, artist.Artist))
	}

	musicPlayer.PlayFile(path)

	// clear the current song from the now playing file list
	os.Create(config.MusicFile)

	return nil
}

func playRnd() {
	for musicPlayer.Playing {
		// select a random artist, with probability adjusted proportionally to the number of songs by that artist (this finally solves the disproportionate frequency of 'The Tea Song' and 'Blessed Are The Teamakers')
		var artist twedia.Artist
		r := rand.Intn(artists.TotalSongs)
		for _, a := range artists.Artists {
			if r < a.TotalSongs {
				artist = a
				break
			}
			r -= a.TotalSongs
		}

		// now select a random album by that artist
		album := artist.Albums[rand.Intn(len(artist.Albums))]

		// and a random song from that album
		song := album.Songs[rand.Intn(len(album.Songs))]

		err := play(artist, album, song)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func stopPlayback() {
	err := musicPlayer.Stop()
	if err != nil {
		log.Println("Error stopping music player:", err)
	}
	err = speechPlayer.Stop()
	if err != nil {
		log.Println("Error stopping speech player:", err)
	}
	os.Create(config.MusicFile)
}

func rewardCallback(r twedia.TwitchRedemption) {
	for _, rewardAction := range config.PointRewards {
		if strings.EqualFold(r.Reward.Title, rewardAction.Title) {
			completeAction(rewardAction.Action)
			return
		}
	}
}

func completeAction(a action) {
	switch a.Type {
	case "song":
		var artist twedia.Artist
		var album twedia.Album
		var song twedia.Song
		if a.Artist != "" {
			for _, ar := range artists.Artists {
				if strings.EqualFold(ar.Artist, a.Artist) {
					artist = ar
					for _, al := range ar.Albums {
						if strings.EqualFold(al.Name, a.Album) {
							album = al
							for _, s := range al.Songs {
								if strings.EqualFold(s.Title, a.Song) {
									song = s
									break
								}
							}
							break
						}
					}
					break
				}
			}

			err := musicPlayer.Stop()
			if err != nil {
				log.Println("Error stopping music player:", err)
			}
			go play(artist, album, song)
		} else {
			err := musicPlayer.Stop()
			if err != nil {
				log.Println("Error stopping music player:", err)
			}
			go playRnd()
		}
	case "tts":
		lastSpeech = time.Now()

		fn := "tts/" + hashString(a.Text) + ".mp3"

		if !exists(fn) {
			// write spoken speech to file
			twedia.SynthesiseText(a.Text, fn)
		}

		// lower the music volume while the TTS occurs...
		musicPlayer.AdjustVolume(-1.0)

		err := speechPlayer.PlayFile(fn)
		if err != nil {
			log.Println("Error playing synthesised speech:", err)
		}

		// and raise it again!
		musicPlayer.AdjustVolume(1.0)
	}
}

func main() {
	r := make(chan bool)

	// Set up Twitch bot
	t = twitch.NewClient(config.Username, "oauth:"+config.OauthToken)

	t.OnNewMessage(func(c string, u twitch.User, m twitch.Message) {
		if time.Since(lastSpeech) > (5 * time.Minute) {
			for _, chatCommand := range config.ChatCommands {
				if strings.EqualFold(strings.Split(m.Text, " ")[0], chatCommand.Trigger) {
					completeAction(chatCommand.Action)
					return
				}
			}
		}
	})

	t.Join(config.Channel)

	t.OnConnect(func() {
		log.Println("Connected to Twitch chat as " + config.Username)
		r <- true
	})

	go func() {
		err := t.Connect()
		if err != nil {
			panic(err)
		}
	}()

	<-r

	go twedia.ListenChannelPoints(channelID, config.ClientID, config.PubsubOauthToken, rewardCallback)

	for {
		fmt.Print("> ")

		var opt string
		var err error
		for {
			reader := bufio.NewReader(os.Stdin)
			opt, err = reader.ReadString('\n')
			if err == nil {
				break
			}
		}
		opt = strings.ToLower(strings.Replace(strings.Replace(opt, "\n", "", -1), "\r", "", -1))
		if opt == "start" {
			musicPlayer.Playing = true
			go playRnd()
		} else if opt == "pause" {
			musicPlayer.TogglePause()
		} else if opt == "skip" {
			err = musicPlayer.Skip()
			if err != nil {
				log.Println("Error skipping song:", err)
			}
		} else if opt == "stop" {
			stopPlayback()
		} else if opt == "select" {
			artist, album, song := twedia.SelectSong(&artists)
			musicPlayer.Playing = true
			go play(*artist, *album, *song)
		} else if opt == "quit" {
			break
		}
	}

	stopPlayback()
	os.Exit(0)
}
