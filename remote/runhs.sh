#!/bin/bash

bash changeconfig.sh && sleep 40 && bash launchinstances.sh $1 1 && sleep 120 && mkdir $3 && bash scphslog.sh $3 && sleep 20 && bash terminateinstances.sh
