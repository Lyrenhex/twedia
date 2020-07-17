# Twitch-Media-Creditor
TODO: description

## Environment Variables

- `TWITCH_BOT_USERNAME` = Twitch account username of the bot
- `TWITCH_CHANNEL_NAME` = Twitch channel name of the streamer
- `TWITCH_MUSIC_DIR` = Path to the directory storing Twitch-friendly songs
- `TWITCH_MUSIC_FILE` = Path to the file to write the current song to (for OBS to read from)
- `TWITCH_OAUTH_TOKEN` = OAuth token for the bot's Twitch account
- `TWITCH_SONG_LINK_DATABASE_URL` = URL to the JSON-formatted list of songs and their appropriate YouTube URL
- `TWITCH_CLIENT_ID` = Client ID of the Twedia application supplied by [dev.twitch.tv](https://dev.twitch.tv)
  - The application should have an OAuth redirect of `http://localhost`
- `TWITCH_CLIENT_SECRET` = Client Secret for the application with ID specified in `TWITCH_CLIENT_ID`
- `TWITCH_PUBSUB_OAUTH_TOKEN` = (User) OAuth token used for Twitch API v5 and PubSub API requests

(This project has expanded to a good few environment variables -- plans to refactor to use fewer and instead store things in a file are underway)