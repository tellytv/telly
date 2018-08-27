# telly

IPTV proxy for Plex Live written in Golang

## ![#f03c15](https://placehold.it/15/f03c15/000000?text=+) THIS IS A PRERELEASE BETA ![#f03c15](https://placehold.it/15/f03c15/000000?text=+)

It is under active develepment and things may change quickly and dramatically.  Please join the discord server if you use this branch and be prepared for some tinkering and breakage.

# Configuration

Here's an example configuration file. **You will need to create this file.**  It should be placed in `/etc/telly/telly.config.toml` or `$HOME/.telly/telly.config.toml` or `telly.config.toml` in the directory that telly is running from.

```toml
[Discovery]                                    # most likely you won't need to change anything here
  Device-Auth = "telly123"                     # These settings are all related to how telly identifies
  Device-ID = 12345678                         # itself to Plex.
  Device-UUID = ""
  Device-Firmware-Name = "hdhomeruntc_atsc"
  Device-Firmware-Version = "20150826"
  Device-Friendly-Name = "telly"
  Device-Manufacturer = "Silicondust"
  Device-Model-Number = "HDTC-2US"
  SSDP = true

[IPTV]
  Streams = 1               # number of simultaneous streams that the telly virtual DVR will provide
                            # This is often 1, but is set by your iptv provider; for example, 
                            # Vaders provides 5
  Starting-Channel = 10000  # When telly assigns channel numbers it will start here
  XMLTV-Channels = true     # if true, any channel numbers specified in your M3U file will be used.
  FFMpeg = true             # if true, streams are buffered through ffmpeg; ffmpeg must be on your $PATH
                            # if you want to use this with Docker, be sure you use the correct docker image
  
[Log]
  Level = "info"            # Only log messages at or above the given level. [debug, info, warn, error, fatal]
  Requests = true           # Log HTTP requests made to telly

[Web]
  Base-Address = "0.0.0.0:6077"   # Set this to the IP address of the machine telly runs on
  Listen-Address = "0.0.0.0:6077" # this can stay as-is

#[SchedulesDirect]           # If you have a Schedules Direct account, fill in details and then
                             # UNCOMMENT THIS SECTION
#  Username = ""             # This is under construction; Vader is the only provider
#  Password = ""             # that works with it fully at this time

[[Source]]
  Name = ""                 # Name is optional and is used mostly for logging purposes
  Provider = "Vaders"       # named providers currently supported are "Vaders", "area51", "Iris"
  Username = "YOUR_IPTV_USERNAME"
  Password = "YOUR_IPTV_PASSWORD"
  Filter = "Sports|Premium Movies|United States.*|USA"
  FilterKey = "group-title" # FilterKey normally defaults to whatever the provider file says is best, 
                            # otherwise you must set this.
  FilterRaw = false         # FilterRaw will run your regex on the entire line instead of just specific keys.
  Sort = "group-title"      # Sort will alphabetically sort your channels by the M3U key provided

[[Source]]
  Name = ""
  Provider = "IPTV-EPG"
  Username = "M3U-Identifier"  # From http://iptv-epg.com/[M3U-Identifier].m3u
  Password = "XML-Identifier"  # From http://iptv-epg.com/[XML-Identifier].xml


[[Source]]
  Provider = "Custom"
  M3U = "http://myprovider.com/playlist.m3u"  # These can be either URLs or fully-qualified paths.
  EPG = "http://myprovider.com/epg.xml"
```
You only need one source; the ones you are not using should be commented out or deleted. The name and filter-related keys can be used with any of the sources.

# FFMpeg

Telly can buffer the streams to Plex through ffmpeg.  This has the potential for several benefits, but today it primarily:

1. Allows support for stream formats that may cause problems for Plex directly.
1. Eliminates the use of redirects and makes it possible for telly to report exactly why a given stream failed.

To take advantage of this, ffmpeg must be installed and available in your path.

# Docker

There are two different docker images available:

## tellytv/telly:dev
The standard docker image for the dev branch

## tellytv/telly:dev-ffmpeg
This docker image has ffmpeg preinstalled.  If you want to use the ffmpeg feature, use this image.  It may be safest to use this image generally, since it is not much larger than the standard image and allows you to turn the ffmpeg features on and off without requiring changes to your docker run command.  The examples below use this image.

## `docker run`
```
docker run -d \
  --name='telly' \
  --net='bridge' \
  -e TZ="America/Chicago" \
  -p '6077:6077/tcp' \
  -v /host/path/to/telly.config.toml:/etc/telly/telly.config.toml \
  --restart unless-stopped \
  tellytv/telly:dev-ffmpeg
```

## docker-compose
```
telly:
  image: tellytv/telly:dev-ffmpeg
  ports:
    - "6077:6077"
  environment:
    - TZ=Europe/Amsterdam
  volumes:
    - /host/path/to/telly.config.toml:/etc/telly/telly.config.toml
  restart: unless-stopped
```

# Troubleshooting

Please free to [open an issue](https://github.com/tellytv/telly/issues) if you run into any problems at all, we'll be more than happy to help.

# Social

We have [a Discord server you can join!](https://discord.gg/bnNC8qX)

