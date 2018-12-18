#!/bin/sh

dep ensure -update
make build
make push
