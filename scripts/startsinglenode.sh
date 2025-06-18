#!/bin/bash
#$1:node id
#$2:node num
#$3:tx speed

# 检查是否提供了参数
if [ $# -eq 0 ]; then
    echo "用法: $0 <循环次数>"
    exit 1
fi

cd ../broadcaster/ && bash startbroadcast.sh $1 $2 1 & cd ../order/ && bash startorderlocal.sh $1 & cd ../tx_pool/ && bash starttxpool.sh $1 & 
#cd ../broadcaster/ && bash startbroadcast.sh $1 $2 1 & cd ../order/ && sudo nice -n -10 bash startorder.sh $1 & cd ../tx_pool/ && bash starttxpool.sh $1 & 
