FROM golang:alpine

COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh && \
    apk add --no-cache git && \
    go get github.com/qvl/ghbackup && \
    go build -o /usr/bin/ghbackup github.com/qvl/ghbackup && \
    rm -R /go/src/github.com && \
    ghbackup -version

ENTRYPOINT ["/entrypoint.sh"]
