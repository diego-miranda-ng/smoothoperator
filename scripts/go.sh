#!/bin/bash
docker-compose -f .devcontainer/docker-compose.yml exec app go "$@"
