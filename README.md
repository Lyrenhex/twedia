# Twitch-Media-Creditor (`twedia`)

A largely self-contained solution for Twitch streamers* to play curated music collections on streams whilst providing credits both onscreen and in chat (with appropriate YouTube URLs).

(* This software was built for a specific use-case. Access to a webserver is currently required, and some steps *may* be somewhat technical...)

## Requirements

- [Golang](https://go.dev)

## Getting started

1. Build the executable file: `go build`
2. Enable the [Google Cloud Text-to-Speech API](https://console.cloud.google.com/apis/api/texttospeech.googleapis.com/overview) and generate a credential file (`.json`).
3. Set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable to the absolute path of the downloaded credential file.
4. Generate a Client ID and Secret from [Twitch Developer console](https://dev.twitch.tv).
5. Create a configuration file following the schema described under `Config file`.
    - You **must** have a music collection metadata file stored on a live webserver; [example](https://lyrenhex.com/stream-content/music.json).
    - `oauthToken` must be generated for the Twitch IRC system; https://twitchapps.com/tmi/ -- access this **using the bot's account**, not your own (create one).
    - `pubsubOauthToken` will be generated on first run of `twedia`; please follow the instructions provided and save the resulting token in the config file.
        - This will expire periodically, re-triggering this process. Please update the saved token when this happens.
        - You **must** restart `twedia` after saving the token in the config file; it should no longer ask for a token.
    - `musicDir`'s directory must be organised such that, matching the music collection in JSON form, each artist has a folder containing folders for each of their albums, each of which contains the relevant songs in `mp3` format.
6. Set the `TWITCH_CONFIG_FILE` environment variable to the absolute path of the newly created configuration file.
7. Run the bot. :>

## Environment Variables

- `TWITCH_CONFIG_FILE` = Absolute path to the configuration file for the bot (json, see below).

## Config file

Subsequent configuration is handled by the config file, which should be structured similarly so:

```json
{
    "username": "botlyren",
    "channel": "Lyrenhex",
    "clientID": "client ID from the Twitch developer site",
    "clientSecret": "client Secret from the Twitch developer site",
    "musicDir": "Absolute path to the folder containing the music (Artist -> Album -> Song.mp3)",
    "musicFile": "Absolute path to the file read by OBS for on-screen music credit.",
    "oauthToken": "OAuth Token for the bot's IRC connection to chat.",
    "pubsubOauthToken": "OAuth Token for the bot's PubSub connection (different to above -- this is generated using the web auth flow on first run, and then can be populated here before restarting the bot)",
    "musicCollectionURL": "https://lyrenhex.com/stream-content/music.json (replace with your own :))",
    "chatCommands": [
        {
            "trigger": "!example",
            "action": {
                "type": "tts",
                "text": "Something for the TTS system to speak! (NB. this uses Google Cloud; please see their docs for the Go library to set this up)"
            }
        }
    ],
    "pointRewards": [
        {
            "rewardTitle": "Play (Specific Song)",
            "action": {
                "type": "song",
                "artist": "Artist name",
                "album": "Album name",
                "title": "Song name"
            }
        },
        {
            "rewardTitle": "TTS Reward",
            "action": {
                "type": "tts",
                "text": "This also supports TTS! See above..."
            }
        }
    ]
}
```
