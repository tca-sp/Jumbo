#!/bin/bash
cd ./golang/src/dumbo_fabric/broadcaster/ && bash start.sh $1 $2 & cd ./golang/src/dumbo_fabric/order/ && bash start.sh $1 & cd ./golang/src/dumbo_fabric/tx_pool/ && bash start.sh $1
