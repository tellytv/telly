# telly

IPTV proxy for Plex Live written in Golang

Please refer to the [Wiki](https://github.com/tellytv/telly/wiki) for the most current documentation.

## This readme refers to version ![#0eaf29](https://placehold.it/15/0eaf29/000000?text=+) 1.1.x ![#0eaf29](https://placehold.it/15/0eaf29/000000?text=+).  It does not apply to versions other than that.

The [Wiki](https://github.com/tellytv/telly/wiki) includes walkthroughs for most platforms that go into more detail than listed below:

## ![#f03c15](https://placehold.it/15/f03c15/000000?text=+) THIS IS A DEVELOPMENT BRANCH ![#f03c15](https://placehold.it/15/f03c15/000000?text=+)

It is under active development and things may change quickly and dramatically.  Please join the discord server if you use this branch and be prepared for some tinkering and breakage.

# Configuration

Here's an example configuration file. **You will need to create this file.**  It should be placed in `/etc/telly/telly.config.toml` or `$HOME/.telly/telly.config.toml` or `telly.config.toml` in the directory that telly is running from.

> NOTE "the directory telly is running from" is your CURRENT WORKING DIRECTORY.  For example, if telly and its config file file are in `/opt/telly/` and you run telly from your home directory, telly will not find its config file because it will be looking for it in your home directory.  If this makes little sense to you, use one of the other two locations OR cd into the directory where telly is located before running it from the command line.

> ATTENTION Windows users: be sure that there isn’t a hidden extension on the file.  Telly can't read its config file if it's named something like `telly.config.toml.txt`.

```toml
# THIS SECTION IS REQUIRED ########################################################################
[Discovery]                                    # most likely you won't need to change anything here
  Device-Auth = "telly123"                     # These settings are all related to how telly identifies
  Device-ID = "12345678"                       # itself to Plex.
  Device-UUID = ""
  Device-Firmware-Name = "hdhomeruntc_atsc"
  Device-Firmware-Version = "20150826"
  Device-Friendly-Name = "telly"
  Device-Manufacturer = "Silicondust"
  Device-Model-Number = "HDTC-2US"
  SSDP = true

# Note on running multiple instances of telly
# There are three things that make up a "key" for a given Telly Virtual DVR:
# Device-ID [required], Device-UUID [optional], and port [required]
# When you configure your additional telly instances, change:
# the Device-ID [above] AND
# the Device-UUID [above, if you're entering one] AND
# the port [below in the "Web" section]

# THIS SECTION IS REQUIRED ########################################################################
[IPTV]
  Streams = 1               # number of simultaneous streams that the telly virtual DVR will provide
                            # This is often 1, but is set by your iptv provider; for example, 
                            # Vaders provides 5
  Starting-Channel = 10000  # When telly assigns channel numbers it will start here
  XMLTV-Channels = true     # if true, any channel numbers specified in your M3U file will be used.
# FFMpeg = true             # if this is uncommented, streams are buffered through ffmpeg; 
                            # ffmpeg must be installed and on your $PATH
                            # if you want to use this with Docker, be sure you use the correct docker image
# if you DO NOT WANT TO USE FFMPEG leave this commented; DO NOT SET IT TO FALSE
  
# THIS SECTION IS REQUIRED ########################################################################
[Log]
  Level = "info"            # Only log messages at or above the given level. [debug, info, warn, error, fatal]
  Requests = true           # Log HTTP requests made to telly

# THIS SECTION IS REQUIRED ########################################################################
[Web]
  Base-Address = "0.0.0.0:6077"   # Set this to the IP address of the machine telly runs on
  Listen-Address = "0.0.0.0:6077" # this can stay as-is

# THIS SECTION IS NOT USEFUL ======================================================================
#[SchedulesDirect]           # If you have a Schedules Direct account, fill in details and then
                             # UNCOMMENT THIS SECTION
#  Username = ""             # This is under construction; no provider
#  Password = ""             # works with it at this time

# AT LEAST ONE SOURCE IS REQUIRED #################################################################
# DELETE OR COMMENT OUT SOURCES THAT YOU ARE NOT USING ############################################
# NONE OF THESE EXAMPLES WORK AS-IS; IF YOU DON'T CHANGE IT, DELETE IT ############################
[[Source]]
  Name = ""                 # Name is optional and is used mostly for logging purposes
  Provider = "Iris"         # named providers currently supported are "area51" and "Iris"
# IF YOUR PROVIDER IS NOT ONE OF THE ABOVE, CONFIGURE IT AS A "Custom" PROVIDER; SEE BELOW
  Username = "YOUR_IPTV_USERNAME"
  Password = "YOUR_IPTV_PASSWORD"
  # THE FOLLOWING KEYS ARE OPTIONAL IN THEORY, REQUIRED IN PRACTICE
  Filter = "YOUR|FILTER|*REGEX"
  FilterKey = "group-title" # FilterKey normally defaults to whatever the provider file says is best, 
                            # otherwise you must set this.
  FilterRaw = false         # FilterRaw will run your regex on the entire line instead of just specific keys.
  Sort = "group-title"      # Sort will alphabetically sort your channels by the M3U key provided

[[Source]]
  Name = ""                    # Name is optional and is used mostly for logging purposes
  Provider = "IPTV-EPG"        # DO NOT CHANGE THIS IF YOU ARE USING THIS PROVIDER
  Username = "M3U-Identifier"  # From http://iptv-epg.com/[M3U-Identifier].m3u
  Password = "XML-Identifier"  # From http://iptv-epg.com/[XML-Identifier].xml
  # NOTE: THOSE KEY NAMES DO NOT MAKE SENSE FOR THIS PROVIDER ################
  # THIS IS JUST AN IMPLEMENTATION DETAIL.  JUST GO WITH IT.
  # For this purpose, IPTV-EPG does not have a "username" and "password", HOWEVER,
  # telly's scaffolding for a "Named provider" does. Rather than special-casing this provider,
  # the username and password are used to hold the two required bits of information.
  # THIS IS JUST AN IMPLEMENTATION DETAIL.  JUST GO WITH IT.
  # NOTE: THOSE KEY NAMES DO NOT MAKE SENSE FOR THIS PROVIDER ################
  # THE FOLLOWING KEYS ARE OPTIONAL HERE; IF YOU"RE USING IPTV-EPG YOU'VE PROBABLY DONE YOUR
  # FILTERING THERE ALREADY
  # Filter = ""
  # FilterKey = ""
  # FilterRaw = false
  # Sort = ""

[[Source]]
  Name = ""                 # Name is optional and is used mostly for logging purposes
  Provider = "Custom"       # DO NOT CHANGE THIS IF YOU ARE ENTERING URLS OR FILE PATHS
                            # "Custom" is telly's internal identifier for this 'Provider'
                            # If you change it to "NAMEOFPROVIDER" telly's reaction will be
                            # "I don't recognize a provider called 'NAMEOFPROVIDER'."
  M3U = "http://myprovider.com/playlist.m3u"  # These can be either URLs or fully-qualified paths.
  EPG = "http://myprovider.com/epg.xml"
  # THE FOLLOWING KEYS ARE OPTIONAL IN THEORY, REQUIRED IN PRACTICE
  Filter = "Sports|Premium Movies|United States.*|USA"
  FilterKey = "group-title" # FilterKey normally defaults to whatever the provider file says is best, 
                            # otherwise you must set this.
  FilterRaw = false         # FilterRaw will run your regex on the entire line instead of just specific keys.
  Sort = "group-title"      # Sort will alphabetically sort your channels by the M3U key provided
# END TELLY CONFIG  ###############################################################################
```
![#f03c15](https://placehold.it/15/f03c15/000000?text=+) You only need one source; the ones you are not using should be commented out or deleted.![#f03c15](https://placehold.it/15/f03c15/000000?text=+)  The name and filter-related keys can be used with any of the sources.

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

