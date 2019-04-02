# Telly

A IPTV proxy for Plex Live written in Golang

A docker create template:

```
docker create --rm \
           --name telly \
           -p 6077:6077 \
           -e PUID=1010 \
           -e PGID=1010 \
           -e STREAMS=1 \
           -e M3U=http://url.com/epg.xml \
           -e EPG=http://url.com/epg.xml \
           -e BASE=127.0.0.1:6077 \
           -e FILTERS="THIS|THAT|THEOTHER" \
           -e FFMPEG=true \
           -v /etc/localtime:/etc/localtime:ro \
           -v /opt/telly:/config \
           telly
```
## Parameters

* `-e PGID` for GroupID 
* `-e PUID` for UserID 
* `-e STREAMS` - Number of simultaneous streams allowed by your IPTV provider
* `-e M3U` - Link provided by your IPTV provider or a full path to a file
* `-e EPG` - Link provided by your IPTV provider or a full path to a file
* `-e BASE` - Set this to the IP address of the machine telly runs on
* `-e FILTER` - A regular expression [or "regex"] that will include entries from the input M3U to get it below 420 channels
* `-e FFMPEG` - Turn FFMPEG to improve plex playback, don't use it if you want to turn off
* `-e PERSISTENCE` - If you customize your config file and don't want to reset it after every restart set it to true
* `-v /opt/telly:/config` - Directory where configuration files are stored
* `-v /etc/localtime:/etc/localtime:ro` - Sync time with host
* `-p *:*` - Ports used, only change the left ports.

**When editing `-v` and `-p` paremeters, the host is always the left and the docker the right. Only change the left**

For shell access while the container is running do `docker exec -it telly bash`.

## Setting up the application 



# To Do List

[nothing yet]

# How to contribute

1. Clone the dev branch with `git clone -b dev https://github.com/Nottt/easy-deluge`
2. Go inside the created directory and build the new docker with `docker build -t telly_dev .`
3. Run it with :
```
docker run --rm \
           --name dev1 \
           -p 7854:8112 \
           -p 60002:58846 \
           -p 60000:50000 \
           -e PUID=1010 \
           -e PGID=1010 \
           -e PASSWORD=password \
           -v ~/deluge-dev/downloads:/downloads \
           -v /etc/localtime:/etc/localtime:ro \
           -v /opt/deluge-dev:/config \
           telly_dev
```
4. Test your features
5. Pull 

OBS: Don't forget to change the ports, folders and --name and clean up the folders if you rebuild the docker after changing stuff

# Known Issues 

