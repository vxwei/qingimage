# 隐私政策 / Privacy Policy — 轻图床助手 (QingImage Helper)

最后更新 / Last updated: 2026-10-23

## 中文

轻图床助手是一款开源浏览器扩展（源码：https://github.com/vxwei/qingimage ），
用于把图片上传到**用户自己搭建的** QingImage 图床。

- **不收集数据**：本扩展不收集、不出售、不向开发者或任何第三方传输任何
  个人信息。我们没有服务器，不做统计、不做跟踪、不含广告。
- **本地设置**：你在设置中填写的图床地址、访问令牌（token）和复制格式
  偏好，只保存在你浏览器本地（chrome.storage.local），不离开你的设备，
  除非你主动卸载或清除浏览器数据。
- **图片上传去向**：你选择上传的图片，只会发送到**你自己在设置中填写的
  图床服务器**，通信在你的浏览器与该服务器之间直连。开发者无法看到、
  也无法控制该服务器上的数据。
- **剪贴板**：上传成功后，扩展仅把生成的图片链接写入你的剪贴板，
  以便你粘贴使用；不读取剪贴板内容。
- **权限用途**：storage 保存设置；contextMenus 提供右键上传菜单；
  clipboardWrite / offscreen 用于在后台把链接复制到剪贴板；notifications
  用于提示上传结果；网站访问权限为可选权限，仅在你保存设置时，
  针对你填写的那一个图床地址申请。
- **联系**：如有疑问，请在 GitHub 仓库提交 Issue。

## English

QingImage Helper is an open-source browser extension
(source: https://github.com/vxwei/qingimage) that uploads images to a
QingImage server **self-hosted by the user**.

- **No data collection**: this extension does not collect, sell, or transmit
  any personal information to the developer or any third party. We run no
  servers, analytics, tracking, or ads.
- **Local settings**: the server address, access token, and copy-format
  preference you enter are stored only in your browser's local storage
  (chrome.storage.local) and never leave your device.
- **Uploads**: images you choose are sent only to the server address you
  configured yourself, directly from your browser to that server.
- **Clipboard**: after an upload, the extension only writes the resulting
  image link to your clipboard; it never reads clipboard contents.
- **Permissions**: storage (settings), contextMenus (right-click upload),
  clipboardWrite / offscreen (copying the link from the background),
  notifications (upload results). Host access is an optional permission,
  requested only for the single server origin you enter in settings.
- **Contact**: please open an issue in the GitHub repository.
