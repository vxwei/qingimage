# QingImage

轻量个人图床：Go 单二进制、零外部依赖。游客免登录即可上传，管理员登录后管理图库。

- 二进制约 7 MB，空载内存占用约 10 MB 级别，适合在 2C/2G 的小机器上与其它服务共存
- 图片存本地磁盘 `data/uploads/年/月/日/`，元数据在 `data/index.json`，备份 = 复制 `data/` 目录

## 权限规则（已定案）

| | 游客（免登录） | 管理员（登录后） |
|---|---|---|
| 上传图片、复制直链 | ✅ | ✅ |
| 看图库 / 历史上传 | ❌ | ✅ |
| 删除、管理、统计 | ❌ | ✅ |
| PicGo / API token | ❌（token 即管理员身份） | ✅ |

游客在当前浏览器里能看到本次会话自己刚传的图（只存在浏览器本地，关页面即消失）。

游客限制（默认值，可用环境变量改）：

- 仅图片：服务端按文件头校验真实类型（jpg/png/gif/webp/avif/bmp/ico），不看后缀，不收 SVG
- 单张 ≤ 10 MB；单 IP 每天 ≤ 20 张、每小时 ≤ 60 张
- 游客独立配额池 2 GB，总容量上限 10 GB（保护小硬盘）
- 文件名为 16 位随机串，链接不可猜测；游客图在图库中带「游客」标记和来源 IP，管理员可一键清理
- 可选游客口令 `IMG_GUEST_PIN`：开启后游客输一次口令、浏览器 30 天免输

## 部署教程（小白照着做）

下面是完整流程，**推荐用方式 A（Docker）**，不用懂 Go、不用手动编译。
全程只在服务器上敲命令，每一步做完都有“应该看到什么”，对不上就先看
[常见问题](#常见问题)再往下走。

### 先准备什么

- 一台 Linux 服务器（VPS/云服务器都行），1 核 1G 内存、20G 硬盘以上就够；
  系统推荐 Debian 12 / Ubuntu 22.04 / Ubuntu 24.04，下面的安装命令按这两个系统写。
- 服务器的公网 IP（在商家后台 / 控制台能看到）。
- 一个域名（可选，没有也能先用 IP 跑起来；想用 `https://img.你的域名`
  才需要，见下面的方式 C）。
- 会用 SSH 连服务器：商家一般给一个 IP、用户名（通常是 `root`）和密码，
  在自己电脑上用终端 / PuTTY / FinalShell 连上去，看到命令行提示符就算连上了。
  后面所有命令都是在**服务器的命令行**里敲，不是在自己电脑上敲。

### 方式 A：Docker 部署（推荐）

#### 第 1 步：检查 Docker 装了没有

```bash
docker --version
docker compose version
```

- 两条都打出版本号（比如 `Docker version 26.x`、`Docker Compose version v2.x`），
  说明已经装了，直接跳到第 2 步。
- 提示 `command not found`，说明没装，继续往下装。

#### 第 2 步：安装 Docker（没装才要做）

Debian / Ubuntu 上执行（每行输完按回车，等它跑完再输下一行）：

```bash
sudo apt update
sudo apt install -y docker.io docker-compose-plugin git nano
sudo systemctl enable --now docker
```

装完再跑一次第 1 步的两条命令，确认都有版本号。

> 如果你不是 root 用户，命令前面要加 `sudo`；如果已经是 root，`sudo` 可以去掉。
> 其它系统（CentOS / Rocky / Alpine 等）请按 [Docker 官方安装文档](https://docs.docker.com/engine/install/)
> 选对应系统安装，装完同样要确认 `docker compose version` 能打出版本号。

#### 第 3 步：下载图床代码

```bash
git clone https://github.com/vxwei/qingimage.git
cd qingimage
ls
```

做完应该看到：`Dockerfile`、`docker-compose.yml`、`main.go`、`web/` 这些文件。
`ls` 里没有这些，说明 `git clone` 没成功，先看报错（一般是网络问题，重试一次；
实在下不下来，也可以在 GitHub 网页上点绿色的 `Code` → `Download ZIP`，解压后传到服务器）。

> 说明：仓库名叫 `qingimage`，所以下载后文件夹也叫 `qingimage`。

#### 第 4 步：设置管理员密码（最重要的一步，先做再启动）

```bash
nano docker-compose.yml
```

打开后找到这一行：

```yaml
      IMG_ADMIN_PASSWORD: "change-me-on-first-run"
```

把双引号里的 `change-me-on-first-run` 改成你自己的密码，**不要留默认的**。
密码建议 12 位以上，字母 + 数字混着来；这个密码只在**第一次启动**时生效，
启动之后再改这个文件就没用了（要改密码得在网页管理界面里改，见第 6 步）。

改完按 `Ctrl+O` → 按回车保存 → 按 `Ctrl+X` 退出 nano。

> 不想用 nano 也可以用 `vi docker-compose.yml`，改完按 `Esc` 再输 `:wq` 回车。
> 改完可以用 `grep IMG_ADMIN_PASSWORD docker-compose.yml` 看一眼，
> 确认显示的是你刚设的密码。

#### 第 5 步：启动

```bash
docker compose up -d --build
```

- `up -d`：后台启动，命令跑完就回到命令行，服务继续在后台跑。
- `--build`：第一次要现场编译镜像，需要几分钟，耐心等它跑完，不要中途按 `Ctrl+C`。

跑完检查一下：

```bash
docker compose ps
docker compose logs --tail 50
```

- `ps` 里应该有一行，`STATUS` 是 `Up`（或者 `Up ... seconds`）。
- 日志里应该有一行 `image-host listening on :8080 (data: /data)`，
  并且**没有**报错（没有 `FATAL` / `failed` / `permission denied`）。
- 如果日志里出现 `FIRST RUN: admin password generated: ...`，
  说明第 4 步的密码没生效（文件没保存、或者改错了地方），
  先停掉再重做第 4 步（见[常见问题](#常见问题)）。

这时服务器上会多出一个 `./data` 目录，图片、图库记录、管理员密码都存在这里，
**备份就是备份这个目录**，删了它图片和账号就全没了。

#### 第 6 步：打开网页，登录管理员

在自己电脑的浏览器里打开：

```text
http://服务器IP:8899
```

把 `服务器IP` 换成你自己的 IP，比如 `http://1.2.3.4:8899`。
能看到图床首页、能上传图片，说明服务已经跑起来了。

然后点页面右上角的「管理员登录」，输入第 4 步设的密码，登录成功后：

1. 往下拉到「修改管理员密码」，把密码再改成一个只有你知道的强密码
   （新密码至少 8 位；改完旧密码就失效了）。
2. 再往下拉到「PicGo / API」，复制里面的 PicGo 配置 / token，
   按[PicGo 配置](#picgo-配置)配好就能从 PicGo 上传了。

> 到这一步，图床已经能用了。想用域名 + HTTPS（正式用建议做），
> 继续看方式 C；只想自己用 IP 访问，也可以先停在这里。

#### 第 7 步（可选）：只让本机访问，留给反向代理

如果后面要套 Nginx 域名（方式 C），不要把 8899 直接暴露在公网上，
改成只允许服务器本机访问。编辑 `docker-compose.yml`，把端口那一行：

```yaml
      - "8899:8080"
```

改成：

```yaml
      - "127.0.0.1:8899:8080"
```

改完重新创建容器让它生效：

```bash
docker compose up -d
```

改完之后直接访问 `http://服务器IP:8899` 会打不开，这是正常的，
后面只能通过域名（Nginx）访问。

### 方式 B：不用 Docker，直接跑二进制

适合服务器上不想装 Docker、或者想用 systemd 托管的人。
需要先在服务器上装 Go（1.27 或更新），或者用一台有 Go 的电脑编好再传过去。

```bash
# 1) 下载代码（同方式 A 第 3 步）
git clone https://github.com/vxwei/qingimage.git
cd qingimage

# 2) 编译（在项目目录里，会生成一个约 7MB 的 image-host 文件）
go build -o image-host .

# 3) 启动（先在前台跑一次，确认没问题再按 Ctrl+C 停掉，改用 systemd）
IMG_ADMIN_PASSWORD=你的密码 IMG_DATA_DIR=./data IMG_PORT=8899 ./image-host
```

前台能看到 `image-host listening on :8899 (data: ./data)`、浏览器能打开，
说明编出来的二进制没问题。再用 systemd 让它开机自启、后台一直跑：

```ini
# 文件位置：/etc/systemd/system/image-host.service
[Unit]
Description=轻图床 image-host
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/image-host
Environment=IMG_DATA_DIR=/opt/image-host/data
Environment=IMG_PORT=8899
Environment=IMG_BIND=127.0.0.1
Environment=IMG_BASE_URL=https://img.example.com
ExecStart=/opt/image-host/image-host
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

- `WorkingDirectory` / `ExecStart` 里的 `/opt/image-host` 改成你实际放二进制的目录；
  二进制和 `data/` 要放在同一个地方（或者把 `IMG_DATA_DIR` 指到数据目录）。
- `IMG_BIND=127.0.0.1` 表示只监听本机，前面套 Nginx 时用；不套反代、想直接用 IP
  访问就把这一行删掉。
- `IMG_BASE_URL` 没有域名时可以删掉这一行，程序会按访问地址自动推导。

装好后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now image-host
sudo systemctl status image-host
```

`status` 显示 `active (running)` 就是跑起来了；看日志用
`sudo journalctl -u image-host -f`。

> 升级二进制时注意一个坑：先给新文件加执行权限再覆盖旧文件，
> 顺序是 `chmod 755 image-host.new` → `mv image-host.new /opt/image-host/image-host`
> → `systemctl restart image-host`。直接 `scp` 覆盖会丢执行权限，服务起不来。

### 方式 C：域名 + HTTPS（反向代理，Nginx + Certbot）

先做完方式 A（第 7 步已把端口改成只监听本机），再做下面这几步。

#### 第 1 步：域名解析到服务器

去域名商家后台，给图床用的子域名（比如 `img.example.com`）加一条 A 记录，
指向服务器的公网 IP。加完等几分钟，在自己电脑上验证：

```bash
ping img.example.com
```

返回的 IP 是你的服务器 IP，才算解析生效了；不是就再等等，或者检查记录填错没有。

#### 第 2 步：装 Nginx 和 Certbot

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

#### 第 3 步：写 Nginx 配置

新建文件 `/etc/nginx/sites-available/img.example.com`
（把 `img.example.com` 换成你自己的域名）：

```nginx
server {
    listen 80;
    server_name img.example.com;

    # 上传文件大，默认的 1m 不够用；这里放宽到 70m（单张 10MB，一次最多 6 张）
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

上面四行 `proxy_set_header` 很重要：没有它们，图床拿不到访客的真实 IP，
游客限流会按反代 IP 算，等于所有人共用一个限额；直链也可能生成 `http://` 而不是 `https://`。

启用并检查：

```bash
sudo ln -s /etc/nginx/sites-available/img.example.com /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

`nginx -t` 要显示 `syntax is ok` 和 `test is successful`，有错先按提示改配置，
不要直接 reload。

#### 第 4 步：申请 HTTPS 证书

```bash
sudo certbot --nginx -d img.example.com
```

按提示输邮箱（用来收证书过期提醒）、同意条款、选是否强制 HTTPS（选是）。
成功后打开 `https://img.example.com`，浏览器地址栏有小锁、能正常上传就成了。
证书 90 天过期，Certbot 会自动续，不用管。

#### 第 5 步：告诉图床对外地址

回到图床目录，编辑 `docker-compose.yml`，把这一行前面的 `#` 去掉，
改成你自己的域名：

```yaml
      IMG_BASE_URL: "https://img.example.com"
```

改完重启容器让它生效：

```bash
docker compose up -d
```

之后生成的图片直链就是 `https://img.example.com/i/xxxx.jpg` 这种。
如果不设 `IMG_BASE_URL`，直链会按访问时的地址推导，大多数情况也能用，
但套了反代 / CDN 后容易生成错地址，所以正式用还是建议填上。

### 备份与恢复

- **备份**：整个 `data/` 目录就是全部数据（图片在 `data/uploads/`，
  图库记录在 `data/index.json`，管理员密码和 token 在 `data/settings.json`）。
  停掉服务再打包最稳妥：

  ```bash
  docker compose down
  tar -czf image-host-backup-$(date +%Y%m%d).tar.gz data/
  docker compose up -d
  ```

  也可以不停车直接打包（正在上传的那一瞬间可能少一张图），
  但恢复时以停机备份为准。
- **恢复**：在新服务器上按方式 A 装好，把备份解压到项目目录覆盖 `data/`，
  再 `docker compose up -d` 即可，管理员密码和 token 都跟着回来。
- **多久备一次**：自己用的话每周备一次、每次改完重要设置后备一次就够了；
  备份文件不要只放同一台服务器上，下载一份到自己电脑或网盘。

### 更新升级

Docker 方式：

```bash
cd qingimage
git pull
docker compose up -d --build
```

`data/` 是挂载出来的，不会被升级清掉；升级完打开网页传一张图验证一下。
二进制方式：重新 `go build`，按方式 B 里说的“先 chmod 再覆盖”换文件并重启服务。

### 常见问题

- **浏览器打不开 `http://服务器IP:8899`**
  先在服务器上跑 `docker compose ps`：`STATUS` 不是 `Up` 就看
  `docker compose logs --tail 100` 的报错；是 `Up` 但打不开，
  多半是防火墙：云服务器要在商家后台安全组放行 8899 端口，
  服务器本机开了 ufw 的话跑 `sudo ufw allow 8899/tcp`。
- **日志里出现 `FIRST RUN: admin password generated: ...`**
  说明启动时没读到你设的密码，第 4 步没改对地方或没保存。
  先 `docker compose down`，重做第 4 步（确认 `grep IMG_ADMIN_PASSWORD docker-compose.yml`
  显示的是你的密码），再 `docker compose up -d --build`。
  注意：这行随机密码只在第一次启动、且没设密码时才打印，之后不会再出现。
- **登录提示“密码不正确”，但密码明明是对的**
  先确认输的是第 4 步设的密码、不是改完之后的新密码；
  登录限速是 10 次 / 10 分钟 / IP，输错太多次要等 10 分钟再试。
- **忘记管理员密码了**
  停掉服务，删掉 `data/settings.json`（只删这一个文件，别删 `data/index.json` 和 `data/uploads/`），
  再启动：程序会当成第一次启动，用 `docker-compose.yml` 里的
  `IMG_ADMIN_PASSWORD` 重新设密码。注意这样会同时重置 PicGo 的 token，
  PicGo 里要重新复制新的 token。
- **上传提示“文件过大” / Nginx 报 413**
  单张上限默认 10 MB（`IMG_MAX_FILE_MB` 可改）；套了 Nginx 的话，
  检查配置里有没有 `client_max_body_size 70m`，没有就加上并 `nginx -t && systemctl reload nginx`。
- **图片打不开、直链是 `http://` 而不是 `https://`**
  检查方式 C 的四行 `proxy_set_header` 是否齐全，
  以及 `docker-compose.yml` 里 `IMG_BASE_URL` 是否填了 `https://` 开头的域名，
  改完重启容器。
- **游客上传不了 / 提示“游客上传需要口令”**
  这是开了 `IMG_GUEST_PIN` 的表现，游客要在页面上先输一次口令；
  不想设口令就把 `docker-compose.yml` 里那一行留空或删掉，重启容器。

## PicGo 配置

PicGo 安装 **lskypro** 插件，选 Lsky Pro 上传器：

- 服务器地址：你的图床地址（如 `https://img.example.com`)
- Token：管理页 PicGo 区复制（含 `Bearer ` 前缀的整串，或按插件要求填纯 token——管理页里给出的是可直接粘贴的配置 JSON）

也可以走通用接口：`POST /api/upload`，表单字段 `file`，请求头 `Authorization: Bearer <token>`，返回 JSON 含直链与 Markdown。

## 环境变量一览

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

## 安全说明

- 管理密码 PBKDF2-SHA256（21 万次迭代）存储；登录限速 10 次/10 分钟/IP
- 上传目录只按图片扩展名出图且带 nosniff，随机文件名与页面分离
- 会话 Cookie 为 HttpOnly + SameSite=Lax，API token 与会话均可吊销（改密码/重生成 token）
