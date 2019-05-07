FROM golang:alpine as builder

# Install git because gin/yaml needs it
# Install make for building
RUN apk update && apk upgrade && apk add git make

# Copy the code from the host and compile it
WORKDIR $GOPATH/src/github.com/tellytv/telly
COPY . ./

# Build the executable using promu since that builds in the version info
# copy the resulting executable to the root under the name "app"
RUN make promu && make build && mv ./telly /app

FROM scratch
# Original: copy from the builder image above:
COPY --from=builder /app ./
EXPOSE 6077
ENTRYPOINT ["./app"]


