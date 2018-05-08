FROM golang:alpine as builder
WORKDIR /go/src/app
ADD . .
# install ca root certificates + listen on 0.0.0.0 + build
RUN apk add --no-cache ca-certificates \
	&& find . -type f -print0 | xargs -0 sed -i 's/localhost/0.0.0.0/g' \
	&& CGO_ENABLED=0 GOOS=linux go install -ldflags '-w -s -extldflags "-static"'

FROM scratch
COPY --from=builder /go/bin/app /app
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
EXPOSE 6077
ENTRYPOINT ["/app", "-temp=/telly.m3u"]
