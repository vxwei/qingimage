# QingImage

轻量个人图床：Go 单二进制、零外部依赖。游客免登录即可上传，管理员登录后管理图库。

- 二进制约 7 MB，空载内存占用约 10 MB 级别，适合在 2C/2G 的小机器上与其它服务共存
- 图片存本地磁盘 `data/uploads/年/月/日/`，元数据在 `data/index.json`，备份 = 复制 `data/` 目录

## 权限规则

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

## 部署（Docker）

```bash
# 在服务器上，项目目录内：
# 1) 先改 docker-compose.yml 里的 IMG_ADMIN_PASSWORD
docker compose up -d --build
# 2) 打开 http://服务器IP:8899 ，点「管理员登录」用上面的密码登录
# 3) 登录后到页面底部修改密码，并在 PicGo 区拿到 API token
```

不用 Docker 也可以直接跑二进制：`IMG_ADMIN_PASSWORD=你的密码 IMG_DATA_DIR=./data ./qingimage`，默认监听 8080（`IMG_PORT` 可改）。首次启动若没给密码，会随机生成并打印在日志里（`docker compose logs` 可见）。

## 域名与反向代理

正式使用建议套域名 + HTTPS。Nginx 示例（图床监听 8899 时）：

```nginx
server {
    listen 443 ssl;
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

配好后在 compose 里设置 `IMG_BASE_URL=https://img.example.com`，生成的直链就用这个域名。

本地如果再套 Cloudflare：DNS 指到服务器、开缓存即可，图片链接带一年强缓存头；回源记得保留上述 X-Forwarded-* 头。

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
