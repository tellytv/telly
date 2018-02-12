FROM golang:alpine as builder
WORKDIR /go/src/app
ADD . .
RUN find . -type f -print0 | xargs -0 sed -i 's/localhost/0.0.0.0/g' \
	&& CGO_ENABLED=0 GOOS=linux go-wrapper install -ldflags '-w -s -extldflags "-static"'

FROM scratch
COPY --from=builder /go/bin/app /app
EXPOSE 6077
ENTRYPOINT ["/app"]
