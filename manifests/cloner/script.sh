#!/bin/sh

retries = 0
max_retries = 5
if [ "$1" == "source" ] ; then
  echo "Starting clone source"
  mkfifo /tmp/clone/socket/pipe
  echo "creating tarball of the image and redirect it to /tmp/clone/socket/pipe "
  tar cv /tmp/clone/image/ > /tmp/clone/socket/pipe
elif [ "$1" == "target" ] ; then
  echo "Starting clone target"
  while true; 
  do
    if [ -e "/tmp/clone/socket/pipe" ]; then
      pushd /tmp/clone/image
      echo "extract the image from /tmp/clone/socket/pipe into /tmp/clone/image directory"
      tar xvf /tmp/clone/socket/pipe
      popd
      break
    elif [ $retries -eq $max_retries ]; then
      echo "retries to clone image has reached maximum retries %s. $max_retries "
      exit 1
    fi
    sleep 3
    retries=$((retries+1)) 
  done
else
  echo "argument value for this script is missing or wrong. shuold be 'source' or 'target'"
  exit 1
fi
