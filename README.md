# Telly

A IPTV proxy for Plex Live written in Golang

A docker create template for *nix systems (see [here for OS X and Windows)](https://github.com/Nottt/telly/blob/master/README.md#os-x-and-windows):

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
* `-e M3U` - Link provided by your IPTV provider or a [full path to a file](https://github.com/Nottt/telly#path-of-m3u-and-epg-files)
* `-e EPG` - Link provided by your IPTV provider or a [full path to a file](https://github.com/Nottt/telly#path-of-m3u-and-epg-files)
* `-e BASE` - IP address or domain that Plex will use to connect to telly (must be reachable by plex)
* `-e FILTER` - A regular expression [or "regex"] that will include entries from the input M3U to get it below 420 channels
* `-e FFMPEG` - Enable FFMPEG to improve plex playback, optional variable, don't use it to turn it off
* `-e PERSISTENCE` - If you need to customize your configuration file for some reason [see here](https://github.com/Nottt/telly#customizing-the-configuration-file)
* `-v /opt/telly:/config` - Directory where configuration files are stored
* `-v /etc/localtime:/etc/localtime:ro` - Sync time with host
* `-p *:*` - Ports used, only change the left ports.

**When editing `-v` and `-p` paremeters, the host is always the left and the docker the right. Only change the left**

For shell access while the container is running do `docker exec -it telly bash`.
To retrieve telly version while the container is running do `docker exec -it telly telly --version`

## Setting up the application 

If you have done everything correctly you should see output similar to this with `docker logs telly`

```
time="2019-04-02T04:03:23Z" level=info msg="Loaded 3 channels into the lineup from "
time="2019-04-02T04:03:23Z" level=info msg="telly is live and on the air!"
time="2019-04-02T04:03:23Z" level=info msg="Broadcasting from http://0.0.0.0:6077/"
time="2019-04-02T04:03:23Z" level=info msg="EPG URL: http://0.0.0.0:6077/epg.xml"
```

If you see this, procceed to [Adding Telly to Plex](https://github.com/tellytv/telly/wiki/Adding-Telly-to-Plex) if not check your variables.

#### Path of M3U and EPG files

If you decide to use a file instead of a URL, you need to start your path with `/config`.
Example: With `-v /opt/telly:/config` your m3u file should be inside `/opt/telly` in your host and your M3U variable should be `-e M3U=/config/file.m3u`

#### OS X and Windows

Windows and OS X platforms does not have `/etc/localtime` to retrieve timezone information, so you need to add a `-e TZ=Europe/Amsterdam` variable to your docker command and remove `-v /etc/localtime:/etc/localtime:ro \`. 

[List of Time Zones here](https://timezonedb.com/time-zones)

#### Customizing the configuration file 

If need to edit the configuration file directly for some reason use the variable `-e PERSISTENCE=true` so the file won't be overwritten. See how [here](https://github.com/tellytv/telly/wiki/Running-Telly%3A-Config-File)

# How to contribute

1. Clone the branch with `git clone -b docker-experimental https://github.com/tellytv/telly`
2. Go inside the created directory and build the new docker with `docker build -t telly_dev .`
3. Run it with :
```
docker create --rm \
           --name dev1 \
           -p 6069:6077 \
           -e PUID=1000 \
           -e PGID=1000 \
           -e STREAMS=1 \
           -e M3U=/config/iptv.m3u \
           -e EPG=/config/epg.xml \
           -e BASE=127.0.0.1:6077 \
           -e FILTERS="THIS|THAT|THEOTHER" \
           -v /etc/localtime:/etc/localtime:ro \
           -v /opt/telly:/config \
           telly_dev
```
5. Run it with `docker start dev1`
6. Test your features
7. Pull 

OBS: Don't forget to change the ports, folders and --name and clean up the folders if you rebuild the docker after changing stuff
