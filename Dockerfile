FROM golang:1.22-alpine

RUN apk add --no-cache git git-lfs

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /usr/local/bin/ghbackup main.go

ENV GITHUB_SECRET=""
ENV CRON_EXPRESSION="0 0 * * *"

VOLUME ["/ghbackup"]

COPY ["entrypoint.sh", "/entrypoint.sh"]

ENTRYPOINT ["/entrypoint.sh"]

LABEL org.opencontainers.image.source https://github.com/digitalpardoe/docker-ghbackup
