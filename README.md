# QingImage

轻量个人图床：Go 单二进制、零外部依赖。游客免登录即可上传，管理员登录后管理图库。

- 二进制约 7 MB，空载内存约 10 MB，适合在 2C/2G 的小机器上与其它服务共存
- 图片存本地磁盘 `data/uploads/年/月/日/`，元数据在 `data/index.json`，备份 = 复制 `data/` 目录

## 权限规则

| | 游客（免登录） | 管理员（登录后） |
|---|---|---|
| 上传图片、复制直链 | ✅ | ✅ |
| 看图库 / 历史上传 | ❌ | ✅ |
| 删除、管理、统计 | ❌ | ✅ |
| PicGo / API token | ❌ | ✅ |

游客只能看到自己刚传的图（存在浏览器本地，关页面就没了）。
默认限制：只收图片（jpg/png/gif/webp/avif/bmp/ico，不收 SVG），单张 ≤ 10MB，
单 IP 每天 ≤ 20 张，游客配额 2GB、总量 10GB。

## 下载编译好的二进制（不用自己编译）

- Linux x86_64（VPS）：[qingimage-linux-amd64](https://github.com/vxwei/qingimage/releases/download/v1.0/qingimage-linux-amd64)
- Linux ARM64（ARM 小鸡）：[qingimage-linux-arm64](https://github.com/vxwei/qingimage/releases/download/v1.0/qingimage-linux-arm64)
- FreeBSD x86_64（Serv00）：[qingimage-freebsd-amd64](https://github.com/vxwei/qingimage/releases/download/v1.0/qingimage-freebsd-amd64)
- 校验值：[SHA256SUMS.txt](https://github.com/vxwei/qingimage/releases/download/v1.0/SHA256SUMS.txt)

下载后 `chmod +x` 就能跑，不用装 Go。

## 方式一：VPS（Docker，5 分钟）

需要一台 Linux 服务器（1 核 1G、20G 硬盘就够）和它的公网 IP。

```bash
# 1. 装 Docker（已装就跳过；Debian / Ubuntu）
sudo apt update && sudo apt install -y docker.io docker-compose-plugin git

# 2. 下载代码
git clone https://github.com/vxwei/qingimage.git
cd qingimage

# 3. 设管理员密码：打开 docker-compose.yml，把 IMG_ADMIN_PASSWORD 改成你的密码
#    （nano 打开后改完按 Ctrl+O 保存、Ctrl+X 退出；只在第一次启动时生效）
nano docker-compose.yml

# 4. 启动
docker compose up -d --build
```

打开 `http://服务器IP:8899`，点「管理员登录」输密码，登录后在页面底部改一次密码，
在「PicGo / API」里拿 token。能传图就成了。

> 不想用 Docker？直接下载上面那个 Linux 二进制，`chmod +x` 后设好环境变量就能跑，见下面折叠区「二进制部署」。
> 打不开？先跑 `docker compose ps`（状态要是 `Up`）和 `docker compose logs --tail 50` 看报错；
> 云服务器还要在安全组放行 8899 端口。

## 方式二：Serv00（免费共享主机）

没有 VPS 也可以装，适合个人轻量用、做备用图床。Serv00 是 FreeBSD，
没有 root、没有 systemd、没有 Docker，所以要多做两步。

```bash
# 1. 在 Serv00 上直接下载 FreeBSD 版，不用自己编译
mkdir -p ~/image-host && cd ~/image-host
curl -LO https://github.com/vxwei/qingimage/releases/download/v1.0/qingimage-freebsd-amd64
chmod +x qingimage-freebsd-amd64
```

2. 在 Serv00 面板里预留一个端口（每户最多 3 个，1024–64000），
   再新建一个 proxy 类型的网站，代理到 `localhost:预留端口`，
   证书在面板里直接签 Let's Encrypt。

3. 在 Serv00 上新建一个启动脚本 `~/image-host/start.sh`：

```bash
#!/bin/sh
export IMG_PORT=预留端口
export IMG_DATA_DIR=$HOME/image-host-data
export IMG_ADMIN_PASSWORD=你的密码
export IMG_TOTAL_QUOTA_GB=2
export IMG_GUEST_QUOTA_GB=1
export IMG_BIND=127.0.0.1
exec $HOME/image-host/qingimage-freebsd-amd64
```

```bash
chmod +x ~/image-host/start.sh
~/image-host/start.sh &
```

4. 保活：Serv00 没有 systemd，进程死了不会自己起来。
   `crontab -e` 加一行，每 5 分钟查一次，没了就拉起来：

```cron
*/5 * * * * pgrep -x qingimage-freebsd-amd64 >/dev/null || $HOME/image-host/start.sh >> $HOME/image-host/image-host.log 2>&1
```

打开 proxy 网站的域名，点「管理员登录」输密码，登录后在页面底部改一次密码，
在「PicGo / API」里拿 token。

> 注意：免费号只有 3GB 盘，所以上面把总量改成了 2GB、游客池 1GB；
> 数据目录（`image-host-data`）要放在网站根目录之外。
> 还有一点要实测：游客限流靠真实 IP，代理没带 `X-Forwarded-For` 的话，
> 所有访客会被当成同一个 IP。

## 域名 + HTTPS（可选）

域名 A 记录指向服务器 IP 后：

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
sudo certbot --nginx -d img.example.com
```

Certbot 会自动配好 HTTPS。之前再加一段 Nginx 反代（`/etc/nginx/sites-available/img.example.com`）：

```nginx
server {
    listen 80;
    server_name img.example.com;
    client_max_body_size 70m;
    location / {
        proxy_pass http://127.0.0.1:8899;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

然后在 `docker-compose.yml` 里把端口改成 `"127.0.0.1:8899:8080"`，
把 `IMG_BASE_URL` 改成 `https://img.example.com`，再 `docker compose up -d`。

<details>
<summary>不用 Docker：直接跑二进制</summary>

```bash
# 直接下载编译好的（不用装 Go）：https://github.com/vxwei/qingimage/releases/download/v1.0/qingimage-linux-amd64
# ARM 用 qingimage-linux-arm64；想自己编就 git clone 仓库再 go build -o image-host .（需要 Go 1.27+）
chmod +x qingimage-linux-amd64
IMG_ADMIN_PASSWORD=你的密码 ./qingimage-linux-amd64   # 监听 8080 端口
```

想开机自启就用 systemd 托管，`ExecStart` 指到二进制、再 `systemctl enable --now image-host`；
升级时先 `chmod 755` 再覆盖旧二进制，然后重启服务。

</details>

<details>
<summary>备份、更新、常见问题</summary>

**备份**：整个 `data/` 目录就是全部数据（图片、图库、密码和 token）。
`tar -czf backup.tar.gz data/` 打包一份，存到自己电脑或网盘；恢复时解压覆盖 `data/` 再启动。

**更新**：`git pull && docker compose up -d --build`，`data/` 不会被清掉。

**打不开**：`docker compose ps` 不是 `Up` 就看日志；是 `Up` 但打不开就是防火墙/安全组没放行 8899。

**密码不对**：登录限速 10 次/10 分钟；忘记密码就停掉服务，删 `data/settings.json` 再启动，
用 `docker-compose.yml` 里的密码重设（PicGo token 会一起重置，要重新复制）。

**上传报文件过大**：单张上限 10MB；套了 Nginx 要加 `client_max_body_size 70m`。

</details>

## PicGo 配置

PicGo 装 **lskypro** 插件，选 Lsky Pro 上传器：服务器地址填你的图床地址，
token 在管理页「PicGo / API」里复制。也可以直接调接口：
`POST /api/upload`，表单字段 `file`，请求头 `Authorization: Bearer <token>`。

<details>
<summary>环境变量</summary>

| 变量 | 默认 | 说明 |
|---|---|---|
| IMG_PORT | 8080 | 监听端口 |
| IMG_DATA_DIR | ./data | 数据目录 |
| IMG_ADMIN_PASSWORD | （随机生成并打日志） | 仅首次初始化生效 |
| IMG_BASE_URL | 按请求推导 | 对外地址，生成直链用 |
| IMG_MAX_FILE_MB | 10 | 单文件上限 |
| IMG_GUEST_DAILY | 20 | 游客每 IP 每日张数 |
| IMG_GUEST_HOURLY | 60 | 游客每 IP 每小时张数 |
| IMG_GUEST_QUOTA_GB | 2 | 游客配额池 |
| IMG_TOTAL_QUOTA_GB | 10 | 总容量上限 |
| IMG_GUEST_PIN | 空（关） | 游客上传口令 |
| IMG_TRUST_PROXY | 1 | 是否信任 X-Forwarded-* 头（反代后面保持 1） |

</details>

## 安全说明

- 管理密码 PBKDF2-SHA256（21 万次迭代）存储；登录限速 10 次/10 分钟/IP
- 上传目录只按图片扩展名出图且带 nosniff，随机文件名与页面分离
- 会话 Cookie 为 HttpOnly + SameSite=Lax，API token 与会话均可吊销（改密码/重生成 token）
