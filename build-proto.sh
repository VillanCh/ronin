#!/bin/sh
set -e

protoc -I ./ ronin.proto --go_out=plugins=grpc:ronin/bp