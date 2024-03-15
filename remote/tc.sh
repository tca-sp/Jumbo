#!/bin/bash

# 获取以太网卡名称
interface=$(ls /sys/class/net | grep -E '^e(n|th)')

if [[ -z "$interface" ]]; then
  echo "未找到合适的以太网卡"
  exit 1
fi

# 设置带宽限制
rate1="1024Mbit"  # 设置带宽限制为 1 Mbps
burst1="128mb"  # 设置 burst 大小为 100 Kbps
latency1="50ms"

rate2="500mbit"  # 设置带宽限制为 1 Mbps
burst2="62mb"  # 设置 burst 大小为 100 Kbps
latency2="100ms"

rate3="100mbit"  # 设置带宽限制为 1 Mbps
burst3="12mb"  # 设置 burst 大小为 100 Kbps
latency3="200ms"

# 使用 tc 命令设置带宽限制
#sudo tc qdisc del dev "$interface" root

#sudo tc qdisc add dev "$interface" root handle 1:0 tbf rate "$rate3"  burst "$burst3" latency 200ms
#sudo tc qdisc add dev "$interface" parent 1:1 handle 10:0 netem latency "$latency3" limit 400000

#sudo tc qdisc add dev "$interface" root netem delay "$latency1" limit 400000 rate "rate1"
#sudo tc qdisc add dev "$interface" root tbf rate "$rate1" burst "$burst1" latency "$latency1"
#for ((i=1; i<=1000; i++))
#do
  #  sleep 15s && sudo tc qdisc change dev "$interface" root tbf rate "$rate2" burst "$burst2" latency "$latency2"
 #   sleep 15s && sudo tc qdisc change dev "$interface" root tbf rate "$rate1" burst "$burst1" latency "$latency1"
#
#done



