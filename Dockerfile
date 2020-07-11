FROM alpine:3.12

RUN apk add --no-cache ruby ruby-json git
RUN gem install octokit

ENV GITHUB_SECRET=""

VOLUME ["/ghbackup"]

COPY ["ghbackup.rb", "/usr/local/bin/ghbackup"]
  
RUN echo '0 0,4,8,12,16,20 * * * /usr/local/bin/ghbackup' > /etc/crontabs/root

CMD ["/usr/sbin/crond", "-f"]
