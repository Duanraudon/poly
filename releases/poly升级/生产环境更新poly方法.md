# 生产环境更新poly方法

编写：赵晨枭

审查：田浩然、张旭东、孙康健



## 更新说明

- 本文档默认读者为运维人员。其他阅读人员如有疑问，可联系开发人员。
- 本方法适用情况：poly hub各节点升级。
- 本次升级内容：
1. poly支持中移链跨链。
## 配套文件介绍：

最新版本poly二进制文件。

请使用命令`md5sum poly`校验二进制文件poly是否为 d2a857b422c82faaacfa62e063ed1986

## 执行前应已准备好的工作

从之前运维poly的人员处获取poly相关的网络资料，如：
-  四个poly节点所在服务器的登录操作权限。
-  获知每个poly节点工作目录，启动停止方式；genesis.json、wallet.dat、wallet1.dat、wallet2.dat、wallet3.dat、二进制poly、docker-compose-poly.yaml文件存放位置；poly运行产生的数据文件所在目录data/的路径；
-  获知每个poly节点日志位置。在需要查看日志时，tail -f 动态查看最新日志内容，了解运行情况。

## 执行步骤

###  一、停止四个poly节点，备份生产环境
进行部署前，请先备份之前的 poly 程序和相关配置。如果此次部署发生意外需要回退，可使用备份还原。
建议备份方法:查看poly日志，确认四个节点块高增加正常，且块高一样（主要看每个节点日志中`CurrentBlockHeight`的值是否一致）， 停止所有poly，备份好四个poly节点中的两个节点环境即可（如宁夏两台机器上选一台，香港两台机器里选一台。如果需要恢复数据，另外两台机器可从这两台机器上考取备份文件）。

1. 进入docker-compose-poly.yaml所在目录，执行命令停止poly。

   ```sh
   docker-compose -f docker-compose-poly.yaml down
   ```
2. 进入poly的数据存放目录data所在目录，执行命令备份data数据。

   ```sh
   cp -r data data.b
   ```
3. 执行命令备份原来poly二进制文件。
   ```sh
   mv poly poly.b
   ```

### 二、上传最新poly
将配套最新poly二进制文件上传到四个节点对应的poly所在目录处，赋予执行权。
  ```sh
   chmod +x poly
  ```

### 三、启动四个poly节点

1. 进入docker-compose-poly.yaml所在目录，执行命令启动poly。

   ```sh
   docker-compose -f docker-compose-poly.yaml up -d
   ```

2. 看每个节点的日志是否正常，如无特别报错error，则本次升级poly完成。
