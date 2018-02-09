FROM golang:alpine as builder
WORKDIR /go/src/app
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go-wrapper install -ldflags '-w -s -extldflags "-static"'

FROM scratch
COPY --from=builder /go/bin/app /app
EXPOSE 6077
ENTRYPOINT ["/app"]
