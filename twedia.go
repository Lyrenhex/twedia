package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gempir/go-twitch-irc"
)

const (
	sampleRate beep.SampleRate = 48000
	bufferSize time.Duration   = time.Second / 10
)

// Song is a structure storing the song title and YouTube URL of a song, both as a string.
type Song struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Album is a structure storing the album name and a dynamic array of Song objects to represent the songs present on an album.
type Album struct {
	Name  string `json:"name"`
	Songs []Song `json:"songs"`
}

// Artist is a structure storing the artist name and a dynamic array of Album objects to represent the artist's albums.
type Artist struct {
	Artist string  `json:"artist"`
	Albums []Album `json:"albums"`
}

// Artists is a structure storing a dynamic array within which to store the Artist objects, to be populated by parsing the JSON data file.
type Artists struct {
	Artists []Artist `json:"artists"`
}

var artists Artists

func init() {
	c := http.Client{
		Timeout: time.Second * 5,
	}
	req, err := http.NewRequest(http.MethodGet, os.Getenv("TWITCH_SONG_LINK_DATABASE_URL"), nil)
	if err != nil {
		log.Fatal(err)
	}

	res, err := c.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(data, &artists)
	if err != nil {
		log.Fatal(err)
	}

	// initialise the speaker to the sampleRate defined in constants
	speaker.Init(sampleRate, sampleRate.N(bufferSize))

	// Seed the random Source such that we don't always listen to Blessed are the Teamakers...
	rand.Seed(time.Now().UnixNano())

	log.Println("Init routine complete.")
}

func main() {
	// Set up Twitch bot
	t := twitch.NewClient(os.Getenv("TWITCH_BOT_USERNAME"), os.Getenv("TWITCH_OAUTH_TOKEN"))
	t.Join(os.Getenv("TWITCH_CHANNEL_NAME"))

	t.OnConnect(func() {
		log.Println("Connected to Twitch chat")
	})

	go func() {
		err := t.Connect()
		if err != nil {
			panic(err)
		}
	}()

	f, err := os.Create(os.Getenv("TWITCH_MUSIC_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for {
		// select a random artist
		artist := artists.Artists[rand.Intn(len(artists.Artists))]

		// now select a random album by that artist
		album := artist.Albums[rand.Intn(len(artist.Albums))]

		// and a random song from that album
		song := album.Songs[rand.Intn(len(album.Songs))]

		f.WriteString(fmt.Sprintf("\n%s, by %s", song.Title, artist.Artist))

		// open the song for playing
		s := string(os.PathSeparator)
		f, err := os.Open(os.Getenv("TWITCH_MUSIC_DIR") + s + artist.Artist + s + album.Name + s + song.Title + ".mp3")
		if err != nil {
			log.Println(err)
			continue
		}
		streamer, format, err := mp3.Decode(f)
		if err != nil {
			log.Fatal(err)
		}
		defer streamer.Close()

		resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

		done := make(chan bool)
		speaker.Play(beep.Seq(resampled, beep.Callback(func() {
			done <- true
		})))

		t.Say(os.Getenv("TWITCH_CHANNEL_NAME"), fmt.Sprintf("Playing %s by %s. Listen on YouTube: %s", song.Title, artist.Artist, song.URL))

		<-done
	}
}
