FROM golang:alpine as builder

# Download and install the latest release of dep
ADD https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 /usr/bin/dep
RUN chmod +x /usr/bin/dep

# Install git because gin/yaml needs it
# Install make for building
RUN apk update && apk upgrade && apk add git make

# Copy the code from the host and compile it
WORKDIR $GOPATH/src/github.com/tellytv/telly
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./

# Build the executable using promu since that builds in the version info
# copy the resulting executable to the root under the name "app"
RUN make promu && make build && mv ./telly /app

FROM scratch
# Original: copy from the builder image above:
COPY --from=builder /app ./
EXPOSE 6077
ENTRYPOINT ["./app"]


