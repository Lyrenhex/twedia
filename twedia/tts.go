package twedia

import (
	"context"
	"log"
	"os"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

// SynthesiseText uses the Google Cloud Text-to-Speech API to generate an MP3 audio file speaking the provided string t, and returns the file name of the created MP3 file.
// NB. this function requires that a valid Google API credential file is present and referred to by the 'GOOGLE_APPLICATION_CREDENTIALS' environment variable.
func SynthesiseText(t string, fn string) {
	// Instantiates a client.
	ctx := context.Background()

	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Perform the text-to-speech request on the text input with the selected
	// voice parameters and audio file type.
	req := texttospeechpb.SynthesizeSpeechRequest{
		// Set the text input to be synthesized.
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: t},
		},
		// Build the voice request, select the language code ("en-US") and the SSML
		// voice gender ("female", since apparently Google broke the neutral voice
		// (aside: ffs, Google)).
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-GB",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_FEMALE,
		},
		// Select the type of audio file you want returned.
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		log.Fatal(err)
	}

	// The resp's AudioContent is binary.
	err = os.WriteFile(fn, resp.AudioContent, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
