#!/bin/sh

retries=0
max_retries=5
sleep_time=3
if [ "$1" == "source" ] ; then
  echo "Starting clone source"
  echo "creating fifo pipe"
  mkfifo /tmp/clone/socket/$2/pipe
  echo "creating tarball of the image and redirecting it to /tmp/clone/socket/$2/pipe"
  tar cv /tmp/clone/image/ > /tmp/clone/socket/$2/pipe
  echo "finished writing image to /tmp/clone/socket/$2/pipe"
elif [ "$1" == "target" ] ; then
  echo "Starting clone target"
  while true; 
  do
    echo "checks if the fifo pipe was created by the cloning source pod"
    if [ -e "/tmp/clone/socket/$2/pipe" ]; then
      pushd /tmp/clone/image
      echo "extract the image from /tmp/clone/socket/$2/pipe into /tmp/clone/image directory"
      tar xv < /tmp/clone/socket/$2/pipe
      popd
      echo "finished reading image from /tmp/clone/socket/$2/pipe and writing it to /tmp/clone/image"
      break
    elif [ $retries -eq $max_retries ]; then
      echo "retries to clone image has reached maximum retries - $max_retries."
      exit 1
    fi
    echo "fifo pipe has not been created by the cloning source pod yet. waiting $sleep_time seconds before checking again...."
    sleep $sleep_time
    retries=$((retries+1)) 
  done
else
  echo "argument value for this script is missing or wrong. shuold be 'source' or 'target'"
  exit 1
fi
