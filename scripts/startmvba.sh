#!/bin/bash
#$1:id
#$2:nodes num
#$3:s num
cd ../mvbaonly/ && sudo nice -n -10 bash start.sh $1 & sleep $4 && sudo bash stop.sh

