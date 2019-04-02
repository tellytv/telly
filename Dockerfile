FROM alpine

# Add s6 script

ADD https://github.com/just-containers/s6-overlay/releases/download/v1.22.1.0/s6-overlay-amd64.tar.gz /tmp/
RUN tar xzf /tmp/s6-overlay-amd64.tar.gz -C /

# Copy S6 init scripts

COPY s6/ /etc

# Add telly executable file

ADD https://github.com/Nottt/telly/raw/master/files/app /usr/bin/telly

# Create necessary folders 

mkdir -p /config && \

# Create user and set permissions

adduser --disabled-login --no-create-home --gecos "" telly && \
usermod -G users telly && \

EXPOSE 6077
VOLUME /config
ENTRYPOINT ["/init"]

