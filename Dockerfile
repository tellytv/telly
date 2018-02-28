FROM golang:onbuild as builder
WORKDIR /go/src/app
ADD . .
RUN find . -type f -print0 | xargs -0 sed -i 's/localhost/0.0.0.0/g' \ #listen on 0.0.0.0
	&& find . -type f -print0 | xargs -0 sed -i 's/\/tmp\//\//g' \ #put temp file in / as /tmp/ doesn't exist in scratch
	&& CGO_ENABLED=0 GOOS=linux go install -ldflags '-w -s -extldflags "-static"' #build

FROM scratch
COPY --from=builder /go/bin/app /app
EXPOSE 6077
ENTRYPOINT ["/app"]
