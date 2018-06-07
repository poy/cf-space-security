#!/bin/bash

PORT=9999 ./proxy &>> /dev/shm/log &
HTTP_PROXY=localhost:9999 ./examples &>> /dev/shm/log&
tail -f /dev/shm/log
