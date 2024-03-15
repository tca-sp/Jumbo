#!/bin/bash
cd ./broadcaster/ && bash start.sh $1 $2 & cd ./order/ && bash start.sh $1 & cd ./tx_pool/ && bash start.sh $1
