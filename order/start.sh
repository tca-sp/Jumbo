#!/bin/bash
num =$1
for ((i=0;i<$num;i++));
do
   ./order -id=$i &> ./log/orderlog$i.txt &    
done
