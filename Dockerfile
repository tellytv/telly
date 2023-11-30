FROM scratch
COPY .build/linux-amd64/telly ./app
EXPOSE 6077
ENTRYPOINT ["./app"]


