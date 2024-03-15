aws ec2 describe-instances --query 'Reservations[*].Instances[*].PublicIpAddress' --output text >tmppubip.txt && sed 's/\t/\n/g' tmppubip.txt > pubip.txt
