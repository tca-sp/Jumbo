#!/bin/bash
export GOPATH=/home/ubuntu/golang && ./order -id=$1 &> ./log/orderlog$i.txt 

