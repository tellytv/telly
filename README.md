# telly

IPTV proxy for Plex Live written in Golang

# Setup

1) Go to the releases page and download the correct version for your Operating System
2) Have the .m3u file on hand from your IPTV provider of choice  
**Any command arguments can also be supplied as environment variables, for example -playlist can also be provided as the PLAYLIST environment variable**
3) Run `telly` with the `-playlist` commandline argument pointing to your .m3u file. (This can be a local file or a URL) For example: `./telly -playlist=/home/github/myiptv.m3u`  
4) If you would like multiple streams/tuners use the `-streams` commandline option. Default is 1. When setting or changing this option, `plexmediaserver` will need to be completely **restarted**. 
5) If you would like `telly` to attempt to the filter the m3u a bit, add the `-filterregex` commandline option. If you would like UK only tv, run `telly` with the `-uktv` commandline option. If you would like to use your own regex, run `telly` with `-useregex <regex>`, for example `-useregex .*UK.*`
6) If `telly` tells you `[telly] [info] listening on ...` - great! Your .m3u file was successfully parsed and `telly` is running. Check below for how to add it into Plex.
7) If `telly` fails to run, check the error. If it's self explanitory, great. If you don't understand, feel free to open an issue and we'll help you out. As of telly v0.4 `sed` commands are no longer needed. Woop!
8) For your IPTV provider m3u, try using option `type=m3u_plus` and `output=ts`.

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

telly is automatically built at the [Docker Hub](https://hub.docker.com/r/tombowditch/telly/)

# Troubleshooting

Please free to open an issue if you run into any issues at all, I'll be more than happy to help.

# Social

We have [a Discord server you can join!](https://discord.gg/bnNC8qX)
