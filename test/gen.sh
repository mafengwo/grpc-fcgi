#!/bin/bash

# generate php sdk
protoc --proto_path=./protos  --php_out=./php/ ./protos/*

# generate client sdk of golang
protoc --proto_path=./protos --go_out=plugins=grpc:client ./protos/*
