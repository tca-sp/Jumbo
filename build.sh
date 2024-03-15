#!/bin/bash
cd ./broadcaster/ && go build & cd ./order/ && go build & cd ./tx_pool/ && go build & cd ./client && go build
