#!/bin/bash

PORT=9999 ./proxy &>> /dev/shm/log &
HTTP_PROXY=localhost:9999 PORT=10000 ./examples &>> /dev/shm/log &
HTTP_PROXY=localhost:9999 BACKEND_PORT=10000 ./reverse-proxy &>> /dev/shm/log &
tail -f /dev/shm/log
