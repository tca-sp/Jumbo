#!/bin/bash
#$1:id
#$2:nodes num
#$3:s num
cd ../fin/ && sudo nice -n -10 bash startfin.sh $1 & cd ../tx_pool/ && bash starttxpool.sh $1 & sleep $4 && sudo bash stop.sh

