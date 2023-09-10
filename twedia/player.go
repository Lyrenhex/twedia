package twedia

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

const (
	sampleRate beep.SampleRate = 48000
	bufferSize time.Duration   = time.Second / 10
)

type Player struct {
	closer  beep.StreamSeekCloser
	ctrl    *beep.Ctrl
	volume  *effects.Volume
	Playing bool
}

func InitSpeaker() error {
	// initialise the speaker to the sampleRate defined in constants
	return speaker.Init(sampleRate, sampleRate.N(bufferSize))
}

func NewPlayer() Player {
	return Player{
		Playing: false,
	}
}

func (p *Player) PlayFile(fn string) error {
	var format beep.Format
	var err error

	mf, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer mf.Close()

	ext := filepath.Ext(fn)
	if ext == ".mp3" {
		p.closer, format, err = mp3.Decode(mf)
	} else if ext == ".wav" {
		p.closer, format, err = wav.Decode(mf)
	} else if ext == ".ogg" {
		p.closer, format, err = vorbis.Decode(mf)
	} else if ext == ".flac" {
		p.closer, format, err = flac.Decode(mf)
	} else {
		log.Println("Unrecognised file type: " + fn)
		return errors.New("Unrecognised file type: " + fn)
	}
	defer p.closer.Close()
	if err != nil {
		return err
	}

	resampled := beep.Resample(4, format.SampleRate, sampleRate, p.closer)

	done := make(chan bool)
	p.ctrl = &beep.Ctrl{
		Streamer: beep.Seq(resampled, beep.Callback(func() {
			done <- true
		})),
		Paused: false,
	}

	p.volume = &effects.Volume{
		Streamer: p.ctrl,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}

	speaker.Play(p.volume)

	<-done

	return nil
}

func (p *Player) TogglePause() {
	if p.closer != nil {
		speaker.Lock()
		p.ctrl.Paused = !p.ctrl.Paused
		speaker.Unlock()
	}
}

func (p *Player) AdjustVolume(deltaVolume float64) {
	if p.closer != nil {
		speaker.Lock()
		p.volume.Volume += deltaVolume
		speaker.Unlock()
	}
}

func (p *Player) Skip() error {
	if p.closer != nil {
		err := p.closer.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Player) Stop() error {
	if p.closer != nil {
		err := p.closer.Close()
		if err != nil {
			return err
		} else {
			p.closer = nil
		}
	}
	p.Playing = false
	return nil
}
