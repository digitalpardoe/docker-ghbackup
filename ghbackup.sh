#!/bin/sh

mount | grep '/ghbackup ' > /dev/null
if [ "$?" -ne 0 ]; then
  echo 'Volume /ghbackup must be mounted'
  exit 1
fi

if [[ -z "${GITHUB_SECRET}" ]]; then
  echo 'Variable GITHUB_SECRET must be defined'
  exit 1
fi

ghbackup -silent -secret $GITHUB_SECRET /ghbackup
if [ "$?" -ne 0 ]; then
  echo 'An error has occured in ghbackup'
  exit 1
fi
