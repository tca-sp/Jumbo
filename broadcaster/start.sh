#!/bin/bash
num=$1
for ((i=0;i<$num;i++));
do
	for ((k=0;k<$num;k++));
	do
	    for j in {1..1}
	    do
	       ./broadcaster -nid=$i -lid=$k -sid=$j &> ./log/broadcastlog$i$k$j.txt &    
		
	    done
	done
done 
