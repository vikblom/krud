#!/usr/bin/bash
set -euxo pipefail

ADDR=$(docker inspect krud-psql | jq -r '.[].NetworkSettings.Networks.bridge.IPAddress')

docker build -t krud .
docker run -it --rm --name krud-http -p "8080:8080" krud -url "postgresql://krud:pass@${ADDR}:5432/krud"
