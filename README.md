# Twitch-Media-Creditor
TODO: description

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
