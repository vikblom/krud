#!/usr/bin/bash
set -euxo pipefail

# TODO: Replace -it --rm with -d --rm?
# Foward port to that unit tests can reach it.
docker run -it --rm \
       --name krud-psql \
       -e POSTGRES_USER=krud \
       -e POSTGRES_PASSWORD=pass \
       -v $PWD/initdb:/docker-entrypoint-initdb.d/ \
       -p 2345:5432 \
       postgres:bullseye

# When data is stable
# -e PGDATA=/var/lib/postgresql/data/pgdata \
# -v $PWD/data:/var/lib/postgresql/data \
