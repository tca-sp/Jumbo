#!/bin/bash
max=5000
num=$1
for ((i=0;i<$num;i++));
do
	for j in {1..1}
	do
		./client -id=$i -max=$max &    
	
	done
done 
