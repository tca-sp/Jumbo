#!/bin/bash

# 使用方法: ./script.sh <yaml文件> <参数1=值1> <参数2=值2> ...

if [ $# -lt 2 ]; then
    echo "使用方法: $0 <yaml文件> <参数1=值1> <参数2=值2> ..."
    exit 1
fi

YAML_FILE="./../config/node.yaml"
shift # 移除文件名参数，剩下的都是键值对

# 检查YAML文件是否存在
if [ ! -f "$YAML_FILE" ]; then
    echo "错误: YAML文件 $YAML_FILE 不存在"
    exit 1
fi

# 创建临时文件
TMP_FILE=$(mktemp)

# 复制原文件到临时文件
cp "$YAML_FILE" "$TMP_FILE"

# 处理每个键值对参数
for arg in "$@"; do
    # 分割键值
    IFS='=' read -r key value <<< "$arg"
    
    # 检查键值格式是否正确
    if [[ -z "$key" || -z "$value" ]]; then
        echo "警告: 忽略无效参数 '$arg'，格式应为key=value"
        continue
    fi
    
    # 在YAML中查找并替换键值
    # 处理可能存在的空格和缩进
    if grep -q "^[[:space:]]*${key}[[:space:]]*:" "$TMP_FILE"; then
        # 使用sed进行替换，保留原有缩进
        sed -i "s/^\([[:space:]]*${key}[[:space:]]*:\).*/\1 ${value}/" "$TMP_FILE"
        echo "已更新: $key = $value"
    else
        echo "警告: 键 '$key' 未在YAML文件中找到"
    fi
done

# 确认是否覆盖原文件
#read -p "确认要将更改写入 $YAML_FILE 吗? [y/N] " confirm
#if [[ "$confirm" =~ ^[Yy]$ ]]; then
mv "$TMP_FILE" "$YAML_FILE"
echo "YAML文件已更新"
#else
#    echo "更改已取消，临时文件保存在 $TMP_FILE"
#fi