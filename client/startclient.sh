#!/bin/bash
max=$2
id=1
snum=$1
for ((j=1;j<=$snum;j++));
do
   sleep 5 && ./client -id=$j -max=$max &>./log/clientlog$j.txt &    

done

