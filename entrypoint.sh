#!/bin/sh

# check mount
mount |grep '/ghbackup ' > /dev/null
if [ "$?" -ne 0 ]
then
  echo 'Usage : docker run -v /host/path:/ghbackup'
  exit 1
fi

# args (github user)
if [ "$#" -ne 1 ]
then
  echo 'Usage : Script <github-user>'
  exit 1
fi

# run ghbackup
ghbackup -silent -account $1 /ghbackup
if [ "$?" -ne 0 ]
then
  echo 'ghbackup error'
  exit 1
fi
