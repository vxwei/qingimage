# 轻图床助手（浏览器扩展）

配合 QingImage 图床使用的 Chrome / Edge 扩展（Manifest V3）：上传图片后自动复制链接。

## 功能

- 弹窗：点击选图 / 拖放图片 / Ctrl+V 粘贴截图 → 上传 → 直链自动复制
- 网页图片右键 →「上传图片到图床」→ 上传并自动复制链接（带通知）
- 复制格式可选：直链 / Markdown / HTML（弹窗里切换，会记住）
- 首次使用在设置页填入你自己的图床地址与 token

## 安装（Chrome 与 Edge 通用，一份文件两边都能装）

1. 解压本扩展到一个固定文件夹（装好后不要移动/删除这个文件夹）。
2. Chrome 打开 `chrome://extensions`，Edge 打开 `edge://extensions`。
3. 打开右上角「开发者模式 / 开发人员模式」。
4. 点「加载已解压的扩展程序 / 加载解压缩的扩展」，选择第 1 步的文件夹。
5. 点扩展图标 →「设置」，填入 token（图床网页后台的「PicGo 接口」token），保存。

## 说明

- token 只保存在浏览器本地（chrome.storage.local），上传时放在
  `Authorization: Bearer <token>` 头里调用图床的 `/api/v1/upload` 接口。
- 在图床后台重新生成 token 后，记得回扩展设置里同步更新。
- 代码结构：`lib.js` 是共用的上传逻辑，`popup.*` 是弹窗，
  `background.js` 是右键菜单，`offscreen.*` 负责在后台复制到剪贴板，
  `options.*` 是设置页。
