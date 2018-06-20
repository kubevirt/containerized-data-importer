#!/bin/sh

if [ "$1" == "source" ] ; then
  mkfifo /tmp/clone/socket/pipe; cat /tmp/clone/image/disk.img > /tmp/clone/socket/pipe
elif [ "$1" == "target" ] ; then
  while true; 
  do
    if [ -e "/tmp/clone/socket/pipe" ]; then
      cat /tmp/clone/socket/pipe  > /tmp/clone/image/disk.img 
      break
    fi
    sleep 3
  done
fi
