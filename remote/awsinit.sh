#!/bin/bash
#$1 num of instances
aws ec2 run-instances --image-id ami-0418e5fa9022889b2 --count $1 --instance-type c6a.2xlarge --security-group-ids launch-wizard-6  --key-name chenghao && aws ec2 describe-instances --filters "Name=instance-type,Values=c6a.2xlarge" "Name=instance-state-name,Values=pending,running" --query 'Reservations[*].Instances[*].PrivateIpAddress' --output text >tmpip.txt && sed 's/\t/\n/g' tmpip.txt > ip.txt && aws ec2 describe-instances --filters "Name=instance-type,Values=c6a.2xlarge" "Name=instance-state-name,Values=pending,running" --query "Reservations[].Instances[].InstanceId" --output text >./instanceids.txt


