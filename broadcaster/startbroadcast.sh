#!/bin/bash
id=$1
num=$2
snum=$3
for ((k=1;k<=$num;k++));
do
    for ((j=1;j<=$snum;j++));
    do
        ./broadcaster -nid=$id -lid=$k -sid=$j &> ./log/broadcastlog$id$k$j.txt &    
    
    done
done

