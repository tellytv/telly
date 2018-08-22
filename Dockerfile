FROM ubuntu:16.04
#FROM golang:alpine as builder

# Download and install the latest release of dep
RUN mkdir /goroot && curl https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz | tar xvzf - -C /goroot --strip-components=1
RUN mkdir /gopath
ENV GOROOT /goroot
ENV GOPATH /gopath
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin
ADD https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 /usr/bin/dep
RUN chmod +x /usr/bin/dep

# Install git because gin/yaml needs it
RUN apt-get install --no-install-recommends -y -q curl build-essential ca-certificates git mercurial bzr
RUN apt -y update && apt -y upgrade && apt -y install git && apt -y install ffmpeg
RUN apt-get -y install software-properties-common python-software-properties
#RUN add-apt-repository ppa:gophers/archive
#RUN apt-get update
#RUN apt-get -y install golang-1.10-go

# Copy the code from the host and compile it
WORKDIR $GOPATH/src/github.com/zenjabba/telly
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .

# install ca root certificates + listen on 0.0.0.0 + build
RUN find . -type f -print0 | xargs -0 sed -i 's/"listen", "localhost/"listen", "0.0.0.0/g' \
  && CGO_ENABLED=0 GOOS=linux go install -ldflags '-w -s -extldflags "-static"'

FROM scratch
COPY --from=builder /app ./
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
EXPOSE 6077
ENTRYPOINT ["./app"]
