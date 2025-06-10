#!/bin/bash
cd ./golang/src/jumbo/broadcaster/ && bash start.sh $1 $2 & cd ./golang/src/jumbo/order/ && bash start.sh $1 & cd ./golang/src/jumbo/tx_pool/ && bash start.sh $1
