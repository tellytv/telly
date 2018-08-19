# telly

IPTV proxy for Plex Live written in Golang

# Configuration

Here's an example configuration file. It should be placed in `/etc/telly/telly.config.toml` or `$HOME/.telly/telly.config.toml` or `telly.config.toml` in the directory that telly is running from.

```toml
[Discovery]
  Device-Auth = "telly123"
  Device-ID = 12345678
  Device-UUID = ""
  Device-Firmware-Name = "hdhomeruntc_atsc"
  Device-Firmware-Version = "20150826"
  Device-Friendly-Name = "telly"
  Device-Manufacturer = "Silicondust"
  Device-Model-Number = "HDTC-2US"
  SSDP = true

[IPTV]
  Streams = 1
  Starting-Channel = 10000
  XMLTV-Channels = true

[Log]
  Level = "info"
  Requests = true

[Web]
  Base-Address = "0.0.0.0:6077"
  Listen-Address = "0.0.0.0:6077"

[SchedulesDirect]
  Username = ""
  Password = ""

[[Source]]
  Name = ""
  Provider = "Vaders"
  Username = ""
  Password = ""
  Filter = "Sports|Premium Movies|United States.*|USA"
  FilterKey = "tvg-name" # FilterKey normally defaults to whatever the provider file says is best, otherwise you must set this.
  FilterRaw = false # FilterRaw will run your regex on the entire line instead of just specific keys.
  Sort = "group-title" # Sort will alphabetically sort your channels by the M3U key provided

[[Source]]
  Name = ""
  Provider = "IPTV-EPG"
  Username = "M3U-Identifier"
  Password = "XML-Identifier"


[[Source]]
  Provider = "Custom"
  M3U = "http://myprovider.com/playlist.m3u"
  EPG = "http://myprovider.com/epg.xml"
```

# Docker

## `docker run`
```
docker run -d \
  --name='telly' \
  --net='bridge' \
  -e TZ="Europe/Amsterdam" \
  -e 'TELLY_CONFIG_FILE'='/telly.config.toml' \
  -p '6077:6077/tcp' \
  -v '/tmp/telly':'/tmp':'rw' \
  tellytv/telly --listen.base-address=localhost:6077
```

## docker-compose
```
telly:
  image: tellytv/telly
  ports:
    - "6077:6077"
  environment:
    - TZ=Europe/Amsterdam
    - TELLY_CONFIG_FILE=/telly.config.toml
  command: -base=telly:6077
  restart: unless-stopped
```

# Troubleshooting

Please free to open an issue if you run into any issues at all, I'll be more than happy to help.

# Social

We have [a Discord server you can join!](https://discord.gg/bnNC8qX)

