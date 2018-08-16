# telly

IPTV proxy for Plex Live written in Golang

# Setup

> **See end of setup section for an important note about channel filtering**

1) Go to the releases page and download the correct version for your Operating System
2) Mark the file as executable for non-windows platforms `chmod a+x <FILENAME>`
3) Rename the file to "telly" if desired; note that from here this readme will refer to "telly"; the file you downloaded is probably called "telly-linux-amd64.dms" or something like that.
**If you do not rename the file, then substitute references here to "telly" with the name of the file you've downloaded.**
**Under Windows, don't forget the `.exe`; i.e. `telly.exe`.**
4) Have the .m3u file on hand from your IPTV provider of choice
**Any command arguments can also be supplied as environment variables, for example --iptv.playlist can also be provided as the TELLY_IPTV_PLAYLIST environment variable**
5) Run `telly` with the `--iptv.playlist` commandline argument pointing to your .m3u file. (This can be a local file or a URL) For example: `./telly --iptv.playlist=/home/github/myiptv.m3u`
6) If you would like multiple streams/tuners use the `--iptv.streams` commandline option. Default is 1. When setting or changing this option, `plexmediaserver` will need to be completely **restarted**.
7) If you would like `telly` to attempt to the filter the m3u a bit, add the `--filter.regex` commandline option. If you would like to use your own regex, run `telly` with `--filter.regex="<regex>"`, for example `--filter.regex=".*UK.*"`
8) In addition to regex, the `--filter.groups` parameter allows for a comma-separated list of channel groups to expose. For example; `--filter.groups="Entertainment UK, Documentaries UK"` will expose all channels belonging to the 'Entertainment UK' and 'Documentaries UK' groups as defined by your playlist. Channels must have a valid `group-title` attribute defined within the m3u playlist for this to work.
9) If `telly` tells you `[telly] [info] listening on ...` - great! Your .m3u file was successfully parsed and `telly` is running. Check below for how to add it into Plex.
10) If `telly` fails to run, check the error. If it's self explanatory, great. If you don't understand, feel free to open an issue and we'll help you out. As of telly v0.4 `sed` commands are no longer needed. Woop!
11) For your IPTV provider m3u, try using option `type=m3u_plus` and `output=ts`.

> **Regex handling changed in 1.0.  `filter.regex` has become blacklist which defaults to blocking everything.  If you are not using a regex to filter your M3U file, you will need to add at a minimum `--regex.inclusive=true` to the command line.  If you do not add this, telly will by default EXCLUDE everything in your M3U.  The symptom here is typically telly seeming to start up just fine but reporting 0 channels. Note this parameter only affects `filter.regex`, `filter.groups` is always an inclusive list.**

# Adding it into Plex

1) Once `telly` is running, you can add it to Plex. **Plex Live requires Plex Pass at the time of writing**
2) Navigate to `app.plex.tv` and make sure you're logged in. Go to Settings -> Server -> Live TV & DVR
3) Click 'Setup' or 'Add'. Plex won't find your device, so press the text to add it manually - input `localhost:6077` (or whatever port you're using - you can change it using the `-listen` commandline argument, i.e. `-listen localhost:12345`)
4) Plex will find your device (in some cases it continues to load but the continue button becomes orange, i.e. clickable. Click it) - select the country in the bottom left and ensure Plex has found the channels. Proceed.
5) Once you get to the channel listing, `telly` currently __doesn't__ have any idea of EPG data so it __starts the channel numbers at 10000 to avoid complications__ with selecting channels at this stage. EPG APIs will come in the future, but for now you'll have to manually match up what `telly` is telling Plex to the actual channel numbers. For UK folk, `Sky HD` is the best option I've found.
6) Once you've matched up all the channels, hit next and Plex will start downloading necessary EPG data.
7) Once that is done, you might need to restart Plex so the telly tuner is not marked as dead.
8) You're done! Enjoy using `telly`. :-)

# Docker

## `docker run`
```
docker run -d \
  --name='telly' \
  --net='bridge' \
  -e TZ="Europe/Amsterdam" \
  -e 'TELLY_IPTV_PLAYLIST'='/home/github/myiptv.m3u' \
  -e TELLY_IPTV_STREAMS=1 \
  -e TELLY_FILTER_REGEX='.*UK.*' \
  -p '6077:6077/tcp' \
  -v '/tmp/telly':'/tmp':'rw' \
  tombowditch/telly --listen.base-address=localhost:6077
```

## docker-compose
```
telly:
  image: tombowditch/telly
  ports:
    - "6077:6077"
  environment:
    - TZ=Europe/Amsterdam
    - TELLY_IPTV_PLAYLIST=/home/github/myiptv.m3u
    - TELLY_FILTER_REGEX='.*UK.*'
    - TELLY_WEB_LISTEN_ADDRESS=telly:6077
    - TELLY_IPTV_STREAMS=1
    - TELLY_DISCOVERY_FRIENDLYNAME=Tuner1
    - TELLY_DISCOVERY_DEVICEID=12345678
  command: -base=telly:6077
  restart: unless-stopped
```


# Troubleshooting

Please free to open an issue if you run into any issues at all, I'll be more than happy to help.

# Social

We have [a Discord server you can join!](https://discord.gg/bnNC8qX)

