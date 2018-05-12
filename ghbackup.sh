#!/bin/sh

mount | grep '/ghbackup ' > /dev/null
if [ "$?" -ne 0 ]
then
  echo 'Volume /ghbackup must be mounted'
  exit 1
fi

if [[ -z "${GITHUB_USER}" ]]; then
  echo 'Variable GITHUB_USER must be defined'
  exit 1
else
fi

if [[ -z "${GITHUB_SECRET}" ]]; then
  echo 'Variable GITHUB_SECRET must be defined'
  exit 1
else
fi

ghbackup -silent -account $GITHUB_USER -secret $GITHUB_SECRET /ghbackup
if [ "$?" -ne 0 ]
then
  echo 'An error has occured in ghbackup'
  exit 1
fi
