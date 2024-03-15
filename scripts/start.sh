#!/bin/bash
#$1:id
#$2:nodes num
#$3:s num
cd ../broadcaster/ && bash startbroadcast.sh $1 $2 $3 & cd ../order/ && sudo nice -n -10 bash startorder.sh $1 & cd ../tx_pool/ && bash starttxpool.sh $1 & sleep $4 && sudo  bash stop.sh
