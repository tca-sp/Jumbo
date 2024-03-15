#!/bin/bash
#$1:nodes num
#$2:s num
#cat ips.txt | while read y
id=1

file="ip.txt"
ips=$(<"$file")
for ip in $ips
do
    (ssh -i chenghao.pem ubuntu@${ip} -tt "ulimit -n 65536 && bash tc.sh & export GOPATH=/home/ubuntu/golang && cd ./golang/src/dumbo_fabric/scripts/ && nohup bash starths.sh ${id} $1 $2")&
    let id++
done
