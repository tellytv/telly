FROM jrottenberg/ffmpeg:4.0-alpine

RUN apk update && apk upgrade && apk add --update --no-cache ca-certificates musl-dev

COPY telly /bin/telly

USER       nobody
EXPOSE     6077
VOLUME     [ "/telly" ]
WORKDIR    /telly
ENTRYPOINT [ "/bin/telly" ]
CMD        [ "--database.file=/telly/telly.db" ]
