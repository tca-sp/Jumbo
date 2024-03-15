#!/bin/bash

bash changeconfig.sh && sleep 20 && bash launchinstances.sh $1 1 && sleep 60 && bash launchclient.sh 1 $2 && sleep 60 && mkdir $3 && bash scplog.sh $3
