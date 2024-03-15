# Jumbo
## AWS Benchmarks
This section provides a step-by-step tutorial explaining how to benchmark Jumbo on [Amazon Web Services (AWS)](https://aws.amazon.com)
### Step 1. Set up your AWS credentials
Set up your AWS credentials to enable programmatic access to your account from your local machine. These credentials will authorize your machine to create, delete, and edit instances on your AWS account programmatically. First of all, [find your 'access key id' and 'secret access key'](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html#cli-configure-quickstart-creds). Then, create a file `~/.aws/credentials` with the following content:
```
[default]
aws_access_key_id = YOUR_ACCESS_KEY_ID
aws_secret_access_key = YOUR_SECRET_ACCESS_KEY
```
### Step 2. Add your SSH public key to your AWS account
You must now [add your SSH public key to your AWS account](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html). This operation is manual (AWS exposes little APIs to manipulate keys) and needs to be repeated for each AWS region that you plan to use. Upon importing your key, AWS requires you to choose a 'name' for your key; ensure you set the same name on all AWS regions. This SSH key will be used by the python scripts to execute commands and upload/download files to your AWS instances.
If you don't have an SSH key, you can create one using [ssh-keygen](https://www.ssh.com/ssh/keygen/):
```
$ ssh-keygen -f ~/.ssh/aws
```

### Step 3. Depoly Jumbo on AWS
1.   Launch an instance on AWS (with Ubuntu 20.04 LTS). If you are not familiar with AWS, you can visit [Get started with Amazon EC2 Linux instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EC2_GetStarted.html#ec2-launch-instance) to get some help.
2.  To connect to the instance you launch in 1, use SSH:
```
    ssh -i your_ssh_key_path ubuntu@public_ip_of_instance
```
3.   When login in instance. First upload codes of Jumbo into instance. 
```
    scp -i your_ssh_key_path -r jumbo_code_path ubuntu@public_ip_of_instance:~/
```
Then install all dependencies as follows:
```
    sudo apt-get update
    sudo snap install go --classic
    sudo apt-get install autoconf
    sudo apt-get install libtool
    sudo apt install make
    sudo apt-get install libgmp-dev
    \\install secp256k1-schnorrsig-batch-verify
    cd secp256k1-schnorrsig-batch-verify
    bash autogen.sh
     ./configure --enable-module-schnorrsig --enable-examples --enable-benchmark
    make
    make check
    sudo make install
    mkdir ~/golang
    mkdir ~/golang/src
    export GOPATH=/home/ubuntu/golang
    go env -w GO111MODULE=auto
    go get -u github.com/klauspost/reedsolomon
    go get -u github.com/herumi/bls-go-binary
    mv ~/dumbo_fabric/ ~/golang/src
```
Then build Jumbo
```
    cd ~/golang/src/dumbo_fabric/
    bash build.sh
```
Create an image using this instance. If you are not familiar with AWS, you can visit [Create an AMI from an Amazon EC2 Instance](https://docs.aws.amazon.com/toolkit-for-visual-studio/latest/user-guide/tkv-create-ami-from-instance.html) to get some help.
4.  Configure the testbed.(/dumbo_fabric/remote/)
	First modify node.yaml, parameters you should modify are following:
```
	Node_num: how many nodes you want to bench
	BatchSize: Max size of a transaction block
	TxSize: size of a transaction in Byte
	SignatureType: signature type, four options: schnorr_aggregate ecdsa schnorr bls
	UsingDumboMVBA: whether using Dumbo-MVBA
	BroadcastType: broadcast type of broadcast layer, three options: CBC WRBC RBC
	MVBAType: MVBA type, two options: normal(for speedingMVBA) and signaturefree
	Interval: latest time to propose a block
```
    Modify awsinit.sh.
```
	--image-id replace with your image id
	--instance-type instance type, we recommend c6a.2xlarge or better
	--key-name your ssh key name
	--security-group-ids you can delete this option if you are not familiar with it
```
5. Run benchmark
	First launch n AWS servers by awsinit.sh (assume n=128)
```
	bash awsinit.sh 128
```
	Then run benchmark by run.sh (or runfin.sh if you want to test fin). Assume n=128,input rate of each client is 1000, benchtime is 130s, and log folder is ./log/
```
	sleep 40s && bash run.sh 128 1000 130 ./log/
```
	After benchmark, protocol logs are store in ./log. You can use caculate.sh to caculate the average latency of consensus.
```
	bash caculate.sh ./log latency.txt
```

Shield: [![CC BY-NC-SA 4.0][cc-by-nc-sa-shield]][cc-by-nc-sa]

This work is licensed under a
[Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License][cc-by-nc-sa].

[![CC BY-NC-SA 4.0][cc-by-nc-sa-image]][cc-by-nc-sa]

[cc-by-nc-sa]: http://creativecommons.org/licenses/by-nc-sa/4.0/
[cc-by-nc-sa-image]: https://licensebuttons.net/l/by-nc-sa/4.0/88x31.png
[cc-by-nc-sa-shield]: https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey.svg