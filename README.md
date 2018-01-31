# telly

IPTV proxy for Plex Live written in Golang

# Setup

1) Go to the releases page and download the correct version for your Operating System
2) Have the .m3u file on hand from your IPTV provider of choice
3) Run `telly` with the `-file` commandline argument pointing to your .m3u file. For example: `./telly -file=/home/github/myiptv.m3u`
4) If you would like `telly` to attempt to the filter the m3u a bit, add the `-useregex` commandline option. If you would like UK only tv, run `telly` with the `-uktv` commandline option
5) If `telly` tells you `[telly] [info] listening on ...` - great! Your .m3u file was successfully parsed and `telly` is running. Check below for how to add it into Plex.
6) If `telly` fails to run, check the error. More than likely it's the formatting of the .m3u file. Common problems can be the format of the run time. Open your .m3u file in your favourite text editor and check lines starting with `#EXTINF:-1` or `#EXTINF:0`. They should be `#EXTINF:-1,` or `#EXTINF:0,` - the comma is the more important part. You can run a simple `sed` command to fix this, something like `sed -i 's/#EXTINF:-1/#EXTINF:-1,/g' myiptv.m3u`. If all else fails, open an issue and I'll be more than happy to help you out.


# Adding it into Plex

1) Once `telly` is running, you can add it to Plex. **This (Plex Live DVR) requires Plex Pass at the time of writing**
2) Navigate to `app.plex.tv` and make sure you're logged in. Go to Settings -> Server -> Live TV & DVR
3) Click 'Setup' or 'Add'. Plex won't find your device, so press the text to add it manually - input `localhost:6077` (or whatever port you're using - you can change it using the `-listen` commandline argument, i.e. `-listen localhost:12345`)
4) Plex will find your device (in some cases it continues to load but the continue button becomes orange, i.e. clickable. Click it) - select the country in the bottom left and ensure Plex has found the channels. Proceed.
5) Once you get to the channel listing, `telly` currently doesn't have any idea of EPG data so it starts the channel numbers at 10000 to avoid complications with selecting channels at this stage. EPG APIs will come in the future, but for now you'll have to manually match up what `telly` is telling Plex to the actual channel numbers. For UK folk, `Freeview` is the best option I've found.
6) Once you've matched up all the channels, hit next and Plex will start downloading necessary EPG data.
7) You're done! Enjoy using `telly`

# Troubleshooting

Please free to open an issue if you run into any issues (_no pun intended_) at all, I'll be more than happy to help.
