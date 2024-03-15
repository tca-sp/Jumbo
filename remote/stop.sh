#!/bin/bash
ps -aux | grep 'broadcaster' | awk '{{print $2}}' | xargs kill -9 & ps -aux | grep 'client' | awk '{{print $2}}' | xargs kill -9 & ps -aux | grep 'order' | awk '{{print $2}}' | xargs kill -9 & ps -aux | grep 'tx_pool' | awk '{{print $2}}' | xargs kill -9 & ps -aux | grep 'tp2fb' | awk '{{print $2}}' | xargs kill -9 
