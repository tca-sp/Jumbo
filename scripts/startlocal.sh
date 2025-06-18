#!/bin/bash
#$1:nodes num
#$2:tx speed for one client

# 检查是否提供了参数
if [ $# -eq 0 ]; then
    echo "用法: $0 <循环次数>"
    exit 1
fi

# 使用参数作为循环次数
for ((i=1; i<=$1; i++)); do
    bash startsinglenode.sh $i $1 &
done

cd ../client/ && bash startclient.sh $1 $2 &