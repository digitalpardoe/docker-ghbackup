FROM alpine:3.16

RUN apk add --no-cache ruby ruby-json git git-lfs
RUN gem install octokit faraday-retry

ENV GITHUB_SECRET=""
ENV CRON_EXPRESSION="0 0 * * *"

VOLUME ["/ghbackup"]

COPY ["entrypoint.sh", "/entrypoint.sh"]
COPY ["ghbackup.rb", "/usr/local/bin/ghbackup"]

ENTRYPOINT ["/entrypoint.sh"]

LABEL org.opencontainers.image.source https://github.com/digitalpardoe/docker-ghbackup
