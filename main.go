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

	tirc "github.com/gempir/go-twitch-irc"
	"github.com/lyrenhex/twedia/twedia"
	"github.com/lyrenhex/twedia/twitch"
	"github.com/lyrenhex/twedia/veadotube"
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
	Trigger string      `json:"trigger"`
	Action  soundAction `json:"action"`
}

type reward struct {
	Title      string      `json:"title"`
	Sound      soundAction `json:"sound"`
	VTubeState string      `json:"vtubeState"`
}

type soundAction struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Song   string `json:"title"`
}

var config Config
var artists twedia.Music
var t *tirc.Client
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

	veadotube.Connect()

	for {
		channelID, err = twitch.GetChannelID(config.PubsubOauthToken, config.ClientID)
		if err == nil {
			break
		} else if err.Error() == "invalid oauth token" {
			config.PubsubOauthToken = twitch.GetOAuthToken(config.ClientID)
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

func playTrack(artist twedia.Artist, album twedia.Album, song twedia.Song) error {
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

	// handle singles: their album name is the same as the song name
	albumName := album.Name
	if albumName == "[Singles]" {
		albumName = song.Title
	}

	path := config.MusicDir + s + artist.Artist + s + albumName + s

	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	found := false
	for _, f := range files {
		fn := strings.ToLower(f.Name())
		if strings.Contains(fn, strings.ToLower(song.Title)) && (strings.Contains(fn, ".mp3") || strings.Contains(fn, ".flac") || strings.Contains(fn, ".ogg") || strings.Contains(fn, ".wav")) {
			path += f.Name()
			found = true
			break
		}
	}
	if !found {
		log.Println("Song file cannot be found: " + path + song.Title)
		return errors.New("Song file cannot be found: " + path + song.Title)
	}

	f.WriteString(fmt.Sprintf("\n%s, by %s", song.Title, artist.Artist))
	if song.URL != "" {
		t.Say(config.Channel, fmt.Sprintf("Playing %s by %s. Listen on YouTube: %s", song.Title, artist.Artist, song.URL))
	} else {
		t.Say(config.Channel, fmt.Sprintf("Playing %s by %s.", song.Title, artist.Artist))
	}

	err = musicPlayer.PlayFile(path)
	if err != nil {
		log.Println("Error playing file "+path+":", err)
	}

	// clear the current song from the now playing file list
	os.Create(config.MusicFile)

	return nil
}

func play(artist *twedia.Artist, album *twedia.Album, song *twedia.Song) {
	for {
		resolvedArtist := artist
		resolvedAlbum := album
		resolvedSong := song
		if resolvedArtist == nil {
			// select a random artist, with probability adjusted proportionally to the number of songs by that artist (this finally solves the disproportionate frequency of 'The Tea Song' and 'Blessed Are The Teamakers')
			r := rand.Intn(artists.TotalSongs)
			for _, a := range artists.Artists {
				if r < a.TotalSongs {
					resolvedArtist = &a
					break
				}
				r -= a.TotalSongs
			}
		}

		if resolvedAlbum == nil {
			// now select a random album by that artist
			r := rand.Intn(resolvedArtist.TotalSongs)
			for _, al := range resolvedArtist.Albums {
				if r < al.TotalSongs {
					resolvedAlbum = &al
					break
				}
				r -= al.TotalSongs
			}
		}

		if resolvedSong == nil {
			// and a random song from that album
			resolvedSong = &resolvedAlbum.Songs[rand.Intn(len(resolvedAlbum.Songs))]
		}

		err := playTrack(*resolvedArtist, *resolvedAlbum, *resolvedSong)
		if err != nil {
			log.Println(err)
			continue
		}
		if !musicPlayer.ContinuingPlayback {
			break
		}
	}
}

func stopPlayback() {
	musicPlayer.ContinuingPlayback = false
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

func rewardCallback(r twitch.Redemption) {
	for _, rewardAction := range config.PointRewards {
		if strings.EqualFold(r.Reward.Title, rewardAction.Title) {
			completeSoundAction(rewardAction.Sound)
			if rewardAction.VTubeState != "" {
				veadotube.SetState(rewardAction.VTubeState)
			}
			return
		}
	}
}

func completeSoundAction(a soundAction) {
	switch a.Type {
	case "start", "select", "song":
		var artist *twedia.Artist = nil
		var album *twedia.Album = nil
		var song *twedia.Song = nil

		if a.Artist != "" {
			for _, ar := range artists.Artists {
				if strings.EqualFold(ar.Artist, a.Artist) {
					artist = &ar
					break
				}
			}
		}

		if artist != nil && a.Album != "" {
			for _, al := range artist.Albums {
				if strings.EqualFold(al.Name, a.Album) {
					album = &al
					break
				}
			}
		}

		if album != nil && a.Song != "" {
			for _, s := range album.Songs {
				if strings.EqualFold(s.Title, a.Song) {
					song = &s
					break
				}
			}
		}

		err := musicPlayer.Stop()
		if err != nil {
			log.Println("Error stopping music player:", err)
		}
		musicPlayer.ContinuingPlayback = a.Type == "start"
		go play(artist, album, song)
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
	t = tirc.NewClient(config.Username, "oauth:"+config.OauthToken)

	t.OnNewMessage(func(c string, u tirc.User, m tirc.Message) {
		if time.Since(lastSpeech) > (5 * time.Minute) {
			for _, chatCommand := range config.ChatCommands {
				if strings.EqualFold(strings.Split(m.Text, " ")[0], chatCommand.Trigger) {
					completeSoundAction(chatCommand.Action)
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

	go twitch.ListenChannelPoints(channelID, config.ClientID, config.PubsubOauthToken, rewardCallback)

main:
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
		switch opt {
		case "start", "select":
			artist, album, song := twedia.SelectSong(&artists)
			musicPlayer.ContinuingPlayback = opt == "start"
			go play(artist, album, song)
		case "pause":
			musicPlayer.TogglePause()
		case "skip":
			err = musicPlayer.Skip()
			if err != nil {
				log.Println("Error skipping song:", err)
			}
		case "stop":
			stopPlayback()
		case "quit":
			break main
		}
	}

	stopPlayback()
	os.Exit(0)
}
