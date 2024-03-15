#!/bin/bash
#$1:nodes num
#$2:s num
#cat ips.txt | while read y
id=1
file="ip.txt"
ips=$(<"$file")

for ip in $ips
do
    (scp -i chenghao.pem ubuntu@${ip}:~/golang/src/dumbo_fabric/client/log/clientlog1.txt ./clientlog2/${id}.txt) &
    let id++
done
