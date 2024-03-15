#!/bin/bash
export GOPATH=/home/ubuntu/golang && ./fin -id=$1 &> ./log/finlog.txt
