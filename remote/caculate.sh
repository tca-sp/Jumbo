#!/bin/bash

output_file1="tmp1.txt"

# 清空或创建output.txt文件
> "$output_file1"

# 定义要读取的文件夹
folder="$1"

# 检查文件夹是否存在
if [ -d "$folder" ]; then
    # 循环读取文件夹中的文件
    for file in "$folder"/*; do
        # 判断是否为普通文件
        if [ -f "$file" ]; then
            # 使用grep命令查找含有字符串"aaa"的行，并将结果保存到变量last_line中
            last_line=$(grep "all latency" "$file" | tail -n 1)

            # 检查last_line是否有值
            if [ -n "$last_line" ]; then
                # 将last_line写入output.txt文件中
                echo "$last_line" >> "$output_file1"
            fi
        fi
    done
else
    echo "文件夹 $folder 不存在"
fi

input_file2="tmp1.txt"
output_file2="tmp2.txt"

# 清空或创建 output.txt 文件
> "$output_file2"

# 逐行读取输入文件
while IFS= read -r line; do
    # 使用 sed 命令删除行中的 aaa 和 bbb
    modified_line=$(echo "$line" | sed 's/all latency: //g' | sed 's/s//g' | sed 's/m//g')

    # 将修改后的行写入到 output.txt 文件中
    echo "$modified_line" >> "$output_file2"
done < "$input_file2"

input_file3="tmp2.txt"
output_file3="tmp3.txt"

# 清空或创建 output.txt 文件
> "$output_file3"

while IFS= read -r line; do
  if (( $(echo "$line > 100" | bc -l) )); then
    line=$(echo "$line / 1000" | bc -l)
  fi
  echo "$line"
done < "$input_file3" > "$output_file3"



input_file4="tmp3.txt"
output_file4="$2"
> "$output_file4"
sum=0
count=0

# 逐行读取输入文件
while IFS= read -r line; do
    # 将行中的浮点数添加到总和中
    sum=$(echo "$sum + $line" | bc)
    (( count++ ))
done < "$input_file4"

# 计算平均值
average=$(echo "scale=2; $sum / $count" | bc)

# 显示平均值
echo "平均值为: $average"
echo "平均值为: $average" >> "$output_file4"
