# GODNSPOD
这是一个类似花生壳动态域名的项目, 用于获取本机的公网 IP, 并使用 dnspod 的 API 自动将域名绑定到本机公网 IP.

和花生壳类似, 你需要先拥有一个公网 IP (IPv4), 然后将你的域名使用dnspod 解析.
一切完成之后, 在 dnspod 控制台中, 生成一个 鉴权用的 token, 详见 [https://support.dnspod.cn/account/5f2d466de8320f1a740d9ff3](https://support.dnspod.cn/account/5f2d466de8320f1a740d9ff3/)

将 token 放到 config.yaml 中即可.

## 关于配置文件 config.yaml

配置文件使用 yaml 格式, 详见 [https://blog.ywwzwb.pw/2019/05/12/yaml/](https://blog.ywwzwb.pw/2019/05/12/yaml/)
配置项如下:

* get_ip_method: 获取公网 IP 的方式, 目前支持两种
  * wanip: 仅路由器支持, 使用设备的 wan 口 IP 地址, 目前仅在我的路由器(ea6500v2, 梅林版本 380.70_0-X7.9.1)上可以正常使用, 其他路由器暂不清楚.
  * ip.cip.cc: 使用 [ip.cip.cc](http://ip.cip.cc) 的接口获取公网 IP 地址, 所有设备以及 docker 环境均可使用.
* refresh_interval: 检查 IP 地址的时间间隔, 单位是秒, 设置为 0 程序将会经运行一次就退出
* token: dnspod 鉴权用的token, 见 [https://support.dnspod.cn/account/5f2d466de8320f1a740d9ff3](https://support.dnspod.cn/account/5f2d466de8320f1a740d9ff3/)
* basedomain: 你的域名, 例如 example.com
* subdomain: 需要设置绑定的子域名, 例如 www. 设置好之后, subdomain.basedomain 将会绑定到你的 IP 地址. 如果需要将 IP 地址直接绑定到 basedomain 上, 请将 subdomain 设置为 `@` , 如果需要设置为泛域名, 可以设置为 `*`

## 关于直接运行

编译后, 使用一个参数 -c 指定你的配置文件路径, 例如 
``` bash
./godnspod -c /tmp/config.yaml
```

## 关于 docker 运行

你需要准备一个配置文件后, 将其映射为容器中的 /config/config.yaml 文件.
例如:
``` bash
docker run --name godnstest -d --mount type=bind,source=/yourpath/config.yaml,target=/config/config.yaml ywwzwb/godnspod
```

## 给路由器 ea6500v2 的编译命令

```bash
export GOOS=linux && export GOARCH=arm && export GOARM=5 && go build
```
如果要缩小体积, 可以编译时去掉调试信息
```bash
export GOOS=linux && export GOARCH=arm && export GOARM=5 && go build -ldflags="-s -w" 
```