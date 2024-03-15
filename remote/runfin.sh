#!/bin/bash

bash changeconfig.sh && sleep 30 && bash launchfininstances.sh $1 1 $3 && sleep 10 && bash launchclient.sh 1 $2 && sleep 120 && mkdir $4 #&& bash scpfinlog.sh $4 && sleep 20 && bash terminateinstances.sh
