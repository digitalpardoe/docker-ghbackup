#!/bin/bash

readonly OUTPUT="ghbackup.tar"

rm $OUTPUT
docker build -t digitalpardoe/ghbackup:latest .
docker save digitalpardoe/ghbackup:latest > $OUTPUT
