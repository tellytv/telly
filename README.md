# telly

IPTV proxy for Plex Live written in Golang

## This is an ![#f92307](https://placehold.it/15/f92307/000000?text=+) unsupported branch ![#f92307](https://placehold.it/15/f92307/000000?text=+).  It is under active development and prereleases based on it [1.5.x] should not be used by anyone who is intolerant of breakage.

# Configuration

This branch uses a web ui for configuration and stored its configuration in a database.  This UI and database are under development and subject to change without notice.

# Docker

## tellytv/telly:v1.5.0
The standard docker image for this branch

## `docker run`
```
docker run -d \
  --name='telly' \
  --net='bridge' \
  -e TZ="America/Chicago" \
  -v ${PWD}/appdata:/etc/telly \
  --restart unless-stopped \
  tellytv/telly:v1.5.0 --database.file=/etc/telly/telly.db
```

# Troubleshooting

Please free to open an issue if you run into any problems at all, we'll be more than happy to help.

## This is an ![#0eaf29](https://placehold.it/15/0eaf29/000000?text=+) unsupported branch ![#0eaf29](https://placehold.it/15/0eaf29/000000?text=+).  It is under active development and prereleases based on it [1.5.x] should not be used by anyone who is intolerant of breakage.

# Social

We have [a Discord server you can join!](https://discord.gg/bnNC8qX)
