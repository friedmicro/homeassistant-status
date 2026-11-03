#!/usr/bin/env bash
./tools/docker --url http://192.168.1.75:9000 homeassistantstatus fetch
./tools/docker --url http://192.168.1.75:9000 homeassistantstatus api
