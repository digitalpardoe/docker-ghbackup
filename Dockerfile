FROM alpine:3.16

RUN apk add --no-cache ruby ruby-json git git-lfs
RUN gem install octokit faraday-retry

ENV GITHUB_SECRET=""

VOLUME ["/ghbackup"]

COPY ["ghbackup.rb", "/usr/local/bin/ghbackup"]

ENTRYPOINT ["/usr/local/bin/ghbackup"]

LABEL org.opencontainers.image.source https://github.com/digitalpardoe/docker-ghbackup