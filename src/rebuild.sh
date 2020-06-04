#!/bin/bash
set -x

docker build . --no-cache -t sevesalm/alley-oop
docker push sevesalm/alley-oop