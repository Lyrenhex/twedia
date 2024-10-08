# Twitch-Media-Creditor (`twedia`)

A largely self-contained solution for Twitch streamers* to play curated music collections on streams whilst providing credits both onscreen and in chat (with appropriate YouTube URLs).

(* This software was built for a specific use-case. Some steps *may* be somewhat technical.)

## Requirements

- [Golang](https://go.dev)

## Getting started

1. Build the executable file: `go build`
2. Enable the [Google Cloud Text-to-Speech API](https://console.cloud.google.com/apis/api/texttospeech.googleapis.com/overview) and generate a credential file (`.json`).
3. Set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable to the absolute path of the downloaded credential file.
4. Generate a Client ID and Secret from [Twitch Developer console](https://dev.twitch.tv).
5. Create a configuration file following the schema described under `Config file`.
    - `musicCollectionURL` must be a music collection metadata file, of a structure similar to this [example](https://lyrenhex.com/stream-content/music.json), specified as either:
        - A fully-qualified URL to a web-accessible resource, beginning with `http` or `https`. Other protocols are not supported at this time.
        - A path to a file, with said path *not* beginning with the string `http`.
    - `oauthToken` must be generated for the Twitch IRC system; https://twitchapps.com/tmi/ -- access this **using the bot's account**, not your own (create one).
    - `pubsubOauthToken` will be generated on first run of `twedia`; please follow the instructions provided in the Twedia console and the webpage that will be opened in your default browser.
        - This will expire periodically, re-triggering this process.
    - `musicDir`'s directory must be organised such that, matching the music collection in JSON form, each artist has a folder containing folders for each of their albums, each of which contains the relevant songs in `mp3` format.
        - Singles should be grouped in the JSON under a `[Singles]` album, and should then be organised such that each single is at `artist/single/single.ext`, where `artist` is the artist name, `single` is the song title, and `ext` is the file extension.
6. Set the `TWITCH_CONFIG_FILE` environment variable to the absolute path of the newly created configuration file.
7. Run the bot. :>

## Environment Variables

- `TWITCH_CONFIG_FILE` = Absolute path to the configuration file for the bot (json, see below).

## Config file

Subsequent configuration is handled by the config file, which should be structured similarly so :

```json
{
    "username": "botlyren",
    "channel": "Lyrenhex",
    "clientID": "client ID from the Twitch developer site",
    "clientSecret": "client Secret from the Twitch developer site",
    "musicDir": "Absolute path to the folder containing the music (Artist -> Album -> Song.mp3)",
    "musicFile": "Absolute path to the file read by OBS for on-screen music credit.",
    "oauthToken": "OAuth Token for the bot's IRC connection to chat.",
    "pubsubOauthToken": "OAuth Token for the bot's PubSub connection (different to above -- this is generated using the web auth flow on first run, and does not need to be included in the config file when first running the application)",
    "musicCollectionURL": "https://lyrenhex.com/stream-content/music.json (replace with your own :) - this may be a local file path!)",
    "chatCommands": [
        {
            "trigger": "!example",
            "action": {
                "type": "tts",
                "text": "Something for the TTS system to speak! (NB. this uses Google Cloud; please see steps 2 and 3, and/or their docs for the Go library, to set this up)"
            }
        }
    ],
    "pointRewards": [
        {
            "title": "Play (Specific Song)",
            "sound": {
                "type": "song",
                "artist": "Artist name",
                "album": "Album name",
                "title": "Song name"
            }
        },
        {
            "title": "TTS Reward",
            "sound": {
                "type": "tts",
                "text": "This also supports TTS! See above..."
            }
        },
        {
            "title": "Set Veadotube Avatar to 'basic'",
            "vtubeState": "basic"
        }
    ]
}
```
