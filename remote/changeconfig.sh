#!/bin/bash
#cat ips.txt | while read y
#for y in `cat ips.txt`
file="ip.txt"
ips=$(<"$file")
rm -f .ssh/known_hosts

for ip in $ips
do
    (scp -o "StrictHostKeyChecking no" -i chenghao.pem ./node.yaml ubuntu@${ip}:~/golang/src/dumbo_fabric/config/)&&(scp -i chenghao.pem ./ip.txt ubuntu@${ip}:~/golang/src/dumbo_fabric/config/IP/broadcast/ && scp -i chenghao.pem ./ip.txt ubuntu@${ip}:~/golang/src/dumbo_fabric/config/IP/order/ && scp -i chenghao.pem ./tc.sh ubuntu@${ip}:~/) &
done
