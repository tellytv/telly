FROM jrottenberg/ffmpeg:4.0-alpine

RUN apk update && apk upgrade && apk add --update --no-cache ca-certificates musl-dev

COPY telly /bin/telly

EXPOSE     6077
ENTRYPOINT ["/bin/telly"]
