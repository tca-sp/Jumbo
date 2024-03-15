#!/bin/bash

bash changeconfig.sh && sleep 30 && bash launchinstances.sh $1 1 $3 && sleep 10 && bash launchclient.sh 1 $2 && sleep $3 && mkdir $4 #&& bash scplog.sh $4 && sleep 20 && bash terminateinstances.sh
