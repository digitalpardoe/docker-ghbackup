#!/bin/ash

echo "${CRON_EXPRESSION} /usr/local/bin/ghbackup > /proc/1/fd/1 2> /proc/1/fd/2" > /etc/crontabs/root

crond -f -d 8
