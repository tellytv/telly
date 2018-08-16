FROM golang:alpine as builder

# Download and install the latest release of dep
ADD https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 /usr/bin/dep
RUN chmod +x /usr/bin/dep

# Install git because gin/yaml needs it
RUN apk update && apk upgrade && apk add git

# Copy the code from the host and compile it
WORKDIR $GOPATH/src/github.com/tellytv/telly
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .

# install ca root certificates + listen on 0.0.0.0 + build
RUN apk add --no-cache ca-certificates \
  && find . -type f -print0 | xargs -0 sed -i 's/"listen", "localhost/"listen", "0.0.0.0/g' \
  && CGO_ENABLED=0 GOOS=linux go install -ldflags '-w -s -extldflags "-static"'

FROM scratch
COPY --from=builder /app ./
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
EXPOSE 6077
ENTRYPOINT ["./app"]
