#!/bin/bash

bash changeconfig.sh && sleep 40 && bash launchinstances.sh $1 1 && sleep 20 && bash launchclient.sh 1 $2 && sleep 120 && mkdir $3 && bash scplog.sh $3
