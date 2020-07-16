package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gempir/go-twitch-irc"
	"github.com/lyrenhex/twedia/twedia"
)

const (
	sampleRate beep.SampleRate = 48000
	bufferSize time.Duration   = time.Second / 10
)

var artists twedia.Music
var streamer beep.StreamSeekCloser
var t *twitch.Client
var playing bool
var channelID string

func init() {
	err := twedia.GetSongs(&artists)
	if err != nil {
		log.Fatal(err)
	}

	// initialise the speaker to the sampleRate defined in constants
	speaker.Init(sampleRate, sampleRate.N(bufferSize))

	token := twedia.GetOAuthToken()
	channelID = twedia.GetChannelID(token)

	// Seed the random Source such that we don't always listen to Blessed are the Teamakers...
	rand.Seed(time.Now().UnixNano())

	log.Println(`Twedia Music Manager
	
Commands:
	start : start playing random music
	skip  : skips the current song
	stop  : stop playing music
	select: play a specific song
	quit  : exit program`)
}

func play(artist twedia.Artist, album twedia.Album, song twedia.Song, f *os.File) error {
	// open the song for playing
	s := string(os.PathSeparator)
	mf, err := os.Open(os.Getenv("TWITCH_MUSIC_DIR") + s + artist.Artist + s + album.Name + s + song.Title + ".mp3")
	if err != nil {
		return err
	}

	var format beep.Format
	streamer, format, err = mp3.Decode(mf)
	if err != nil {
		return err
	}
	defer streamer.Close()

	f.WriteString(fmt.Sprintf("\n%s, by %s", song.Title, artist.Artist))
	t.Say(os.Getenv("TWITCH_CHANNEL_NAME"), fmt.Sprintf("Playing %s by %s. Listen on YouTube: %s", song.Title, artist.Artist, song.URL))

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done
	return nil
}

func playRnd(f *os.File) {
	for playing {
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

		err := play(artist, album, song, f)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func rewardCallback(rName string, f *os.File) {
	if strings.ToLower(rName) == "play the tea song" {
		var artist twedia.Artist
		var album twedia.Album
		var song twedia.Song
		for _, ar := range artists.Artists {
			if ar.Artist != "Miscellaneous" {
				continue
			}
			artist = ar
			for _, al := range ar.Albums {
				if al.Name != "Yorkshire Tea" {
					continue
				}
				album = al
				for _, s := range al.Songs {
					if s.Title != "The Tea Song" {
						continue
					}
					song = s
				}
			}
		}

		if streamer != nil {
			streamer.Close()
		}
		playing = true
		go play(artist, album, song, f)
	}
}

func main() {
	r := make(chan bool)

	f, err := os.Create(os.Getenv("TWITCH_MUSIC_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Set up Twitch bot
	t = twitch.NewClient(os.Getenv("TWITCH_BOT_USERNAME"), "oauth:"+os.Getenv("TWITCH_OAUTH_TOKEN"))
	t.Join(os.Getenv("TWITCH_CHANNEL_NAME"))

	t.OnConnect(func() {
		log.Println("Connected to Twitch chat")
		r <- true
	})

	go func() {
		err := t.Connect()
		if err != nil {
			panic(err)
		}
	}()

	<-r

	go twedia.ListenChannelPoints(channelID, f, rewardCallback)

	for {
		fmt.Print("> ")

		var opt string
		for {
			reader := bufio.NewReader(os.Stdin)
			opt, err = reader.ReadString('\n')
			if err == nil {
				break
			}
		}
		opt = strings.ToLower(strings.Replace(strings.Replace(opt, "\n", "", -1), "\r", "", -1))
		if opt == "start" {
			playing = true
			go playRnd(f)
		} else if opt == "skip" {
			streamer.Close()
		} else if opt == "stop" {
			streamer.Close()
			playing = false
		} else if opt == "select" {
			artist, album, song := twedia.SelectSong(&artists)
			playing = true
			go play(*artist, *album, *song, f)
		} else if opt == "quit" {
			break
		}
	}

	// clear the contents of the Now Playing file
	os.Create(os.Getenv("TWITCH_MUSIC_FILE"))
}
