package twedia

import (
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

const (
	sampleRate beep.SampleRate = 48000
	bufferSize time.Duration   = time.Second / 10
)

type Player struct {
	streamer beep.StreamSeekCloser
	Playing  bool
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

	p.streamer, format, err = mp3.Decode(mf)
	if err != nil {
		return err
	}
	defer p.streamer.Close()

	resampled := beep.Resample(4, format.SampleRate, sampleRate, p.streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func (p *Player) Skip() error {
	if p.streamer != nil {
		err := p.streamer.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Player) Stop() error {
	if p.streamer != nil {
		err := p.streamer.Close()
		if err != nil {
			return err
		} else {
			p.streamer = nil
		}
	}
	p.Playing = false
	return nil
}
