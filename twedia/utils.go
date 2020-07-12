package twedia

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
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
	Artist     string  `json:"artist"`
	Albums     []Album `json:"albums"`
	TotalSongs int
}

// Music is a structure storing a dynamic array within which to store the Artist objects, to be populated by parsing the JSON data file.
type Music struct {
	Artists    []Artist `json:"artists"`
	TotalSongs int
}

func GetSongs(a *Music) error {
	c := http.Client{
		Timeout: time.Second * 5,
	}
	req, err := http.NewRequest(http.MethodGet, os.Getenv("TWITCH_SONG_LINK_DATABASE_URL"), nil)
	if err != nil {
		return err
	}

	res, err := c.Do(req)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, a)
	if err != nil {
		return err
	}

	for i, ar := range (*a).Artists {
		for _, al := range ar.Albums {
			for range al.Songs {
				(*a).TotalSongs++
				(*a).Artists[i].TotalSongs++
			}
		}
	}

	return nil
}

func SelectSong(artists *Music) (*Artist, *Album, *Song) {
	var artist *Artist
	var album *Album
	var song *Song

	var sAr string
	var err error
	for artist == nil {
		reader := bufio.NewReader(os.Stdin)
		sAr, err = reader.ReadString('\n')
		if err != nil {
			continue
		}
		sAr = strings.ToLower(strings.Replace(strings.Replace(sAr, "\n", "", -1), "\r", "", -1))

		for _, ar := range (*artists).Artists {
			if strings.ToLower(ar.Artist) != sAr {
				log.Println("Could not find that artist.")
				continue
			}
			artist = &ar
		}
	}
	var sAl string
	for album == nil {
		reader := bufio.NewReader(os.Stdin)
		sAl, err = reader.ReadString('\n')
		if err != nil {
			continue
		}
		sAl = strings.ToLower(strings.Replace(strings.Replace(sAl, "\n", "", -1), "\r", "", -1))

		for _, al := range artist.Albums {
			if strings.ToLower(al.Name) != sAl {
				log.Println("Could not find that album.")
				continue
			}
			album = &al
		}
	}
	var sSong string
	for song == nil {
		reader := bufio.NewReader(os.Stdin)
		sSong, err = reader.ReadString('\n')
		if err != nil {
			continue
		}
		sSong = strings.ToLower(strings.Replace(strings.Replace(sSong, "\n", "", -1), "\r", "", -1))

		for _, s := range album.Songs {
			if strings.ToLower(s.Title) != sSong {
				log.Println("Could not find that song.")
				continue
			}
			song = &s
		}
	}

	return artist, album, song
}
