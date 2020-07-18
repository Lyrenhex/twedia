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

var lastSpeech time.Time = time.Unix(0, 0)

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

	fmt.Println(`Twedia Music Manager
	
Commands:
	start : start playing random music
	skip  : skips the current song
	stop  : stop playing music
	select: play a specific song
	quit  : exit program`)
}

func play(artist twedia.Artist, album twedia.Album, song twedia.Song) error {
	var f *os.File
	var err error
	for {
		f, err = os.Create(os.Getenv("TWITCH_MUSIC_FILE"))
		if err == nil {
			break
		}
	}
	defer f.Close()

	// open the song for playing
	s := string(os.PathSeparator)

	f.WriteString(fmt.Sprintf("\n%s, by %s", song.Title, artist.Artist))
	t.Say(os.Getenv("TWITCH_CHANNEL_NAME"), fmt.Sprintf("Playing %s by %s. Listen on YouTube: %s", song.Title, artist.Artist, song.URL))

	err = playFile(os.Getenv("TWITCH_MUSIC_DIR") + s + artist.Artist + s + album.Name + s + song.Title + ".mp3")

	// clear the current song from the now playing file list
	// (this implementation will double line count but is otherwise
	// reasonably cheap from a storage perspective)
	os.Create(os.Getenv("TWITCH_MUSIC_FILE"))

	return nil
}

func playFile(fn string) error {
	var format beep.Format
	var err error

	mf, err := os.Open(fn)
	if err != nil {
		return err
	}

	streamer, format, err = mp3.Decode(mf)
	if err != nil {
		return err
	}
	defer streamer.Close()

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func playRnd() {
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

		err := play(artist, album, song)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func stopPlayback() {
	streamer.Close()
	playing = false
	os.Create(os.Getenv("TWITCH_MUSIC_FILE"))
}

// TODO: eventually find a method by which to generalise this function.
// ideally, we want some kind of text-file interface by which to define songs
// to play / actions to fulfil based on the reward data...
func rewardCallback(rName string) {
	if strings.ToLower(rName) == "play the tea song" {
		var artist twedia.Artist
		var album twedia.Album
		var song twedia.Song
		for _, ar := range artists.Artists {
			if ar.Artist != "Yorkshire Tea" {
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
		go play(artist, album, song)
	}
}

func main() {
	r := make(chan bool)

	// Set up Twitch bot
	t = twitch.NewClient(os.Getenv("TWITCH_BOT_USERNAME"), "oauth:"+os.Getenv("TWITCH_OAUTH_TOKEN"))

	t.OnPrivateMessage(func(m twitch.PrivateMessage) {
		if strings.ToLower(strings.Split(m.Message, " ")[0]) == "!joe" && time.Now().Sub(lastSpeech) > (5*time.Minute) {
			wasPlaying := playing

			// write spoken speech to file
			fn := twedia.SynthesiseText("Remember Damo, Joe is the most rational human on planet Earth, and would never actively be disruptive!")

			stopPlayback()

			err := playFile(fn)
			if err != nil {
				log.Println("Error playing synthesised speech:", err)
			}
			lastSpeech = time.Now()

			if wasPlaying {
				playRnd()
			}
		}
	})

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

	go twedia.ListenChannelPoints(channelID, rewardCallback)

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
			playing = true
			go playRnd()
		} else if opt == "skip" {
			streamer.Close()
		} else if opt == "stop" {
			stopPlayback()
		} else if opt == "select" {
			artist, album, song := twedia.SelectSong(&artists)
			playing = true
			go play(*artist, *album, *song)
		} else if opt == "quit" {
			break
		}
	}

	stopPlayback()
}
