FROM jrottenberg/ffmpeg:4.0-alpine
COPY --from=tellytv/telly:dev /app /app
EXPOSE 6077
ENTRYPOINT ["/app"]
