#!/bin/bash

bash changeconfig.sh && sleep 30 && bash launchmvbainstances.sh $1 1 $3 && sleep 10 && bash launchclient.sh 1 $2 && sleep 120 && mkdir $4 #&& bash scpmvbalog.sh $4 && sleep 20 && bash terminateinstances.sh
