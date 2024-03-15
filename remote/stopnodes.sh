#!/bin/bash
#$1:s num
#$2:tx num
#cat ips.txt | while read y
file="ip.txt"
ips=$(<"$file")

for ip in $ips
do
    ssh -i chenghao.pem ubuntu@${ip} -tt "export GOPATH=/home/ubuntu/golang && cd ./golang/src/dumbo_fabric/scripts/ && (nohup sudo bash stop.sh )"&
done
