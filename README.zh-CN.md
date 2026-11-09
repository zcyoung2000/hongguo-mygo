# myguo

基于 Go 构建的原生 macOS 短剧客户端。

[English](README.md) | [简体中文](README.zh-CN.md)

## 项目简介

myguo 现在以 MyGo GPU 自绘的原生 UI 作为应用主体：侧栏导航、搜索、目录、详情、选集以及本地历史/收藏/预约直接调用既有 Go 服务。播放区域保留嵌入 WebView，让有声播放、切集、进度、收藏关联、画中画和媒体控制继续复用成熟链路；媒体仍经由本地 MediaServer 与 OpenHongguo SDK。

本项目仅供个人技术研究与本地使用，与上游内容平台没有隶属或认可关系。

## 功能特性

| 分类 | 说明 |
| --- | --- |
| 原生桌面界面 | MyGo `ui` GPU 自绘外壳，包含隐藏式标题栏、侧边栏导航、暗色主题、响应式/虚拟化网格和 SVG 图标。 |
| 内容发现 | 推荐、搜索与搜索建议、筛选、排行榜、新剧日历、演员主页和剧集详情。 |
| 本地资料库 | 历史、收藏、点赞和预约保存在 SQLite；原生主体通过既有服务直接读写这些真实记录。 |
| 评论与弹幕 | 剧集评论、回复、时间轴弹幕播放、发送和点赞。 |
| 播放体验 | 剧集列表、画质选择、进度保存、自动连播和键盘控制。 |
| 媒体管线 | 本地服务代理视频，使用 FFmpeg 解密受保护流，并支持 HTTP Range 请求。 |
| 本地持久化 | SQLite 以 WAL 模式保存登录凭据、设备身份、资料和用户数据，并支持旧数据自动导入。 |
| 无 CGO 构建 | Go 运行时和 SQLite 驱动均可在 `CGO_ENABLED=0` 下构建。 |

## 环境要求

- macOS 12 或更高版本
- Go 1.27 或更高版本
- 获取内置 FFmpeg 时需要 Bash、curl、unzip 和 `shasum`
- 开发播放需要 FFmpeg；发布包会内置静态 FFmpeg

## 快速开始

```bash
git clone https://github.com/zcyoung2000/hongguo-mygo.git
cd hongguo-mygo
go mod tidy
go run .
```

### 构建应用包

```bash
./scripts/fetch-ffmpeg.sh
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo build -skip-dmg
```

构建产物位于 Apple Silicon 的 `dist/darwin-arm64/myguo.app`，或 Intel 的 `dist/darwin-amd64/myguo.app`。

## 账号登录

myguo 可以使用匿名设备配置运行。如需同步个人数据：

1. 打开 **设置**。
2. 选择 **立即登录**。
3. 粘贴账号 Cookie。
4. 可选填写 `x-tt-token`，用于获得更长效的会话。

如果本机存在兼容的旧安装登录凭据，应用也会提供一键导入。Cookie 和 Token 应按密码同等对待：请勿提交、分享或泄露。

## 数据与存储

应用数据保存在 `~/Library/Application Support/myguo/`。SQLite 保存登录凭据、设备身份、个人资料、历史、收藏、点赞和预约；应用自身不再创建 `account.json`。已缓存媒体和转换后的图片单独存放，可以在设置中清理。

## 架构

```text
MyGo 原生主体（`ui.View`）
        |
        +--> Go 服务 --> OpenHongguo SDK 与请求辅助层
        |         |
        |         +--> SQLite 用户数据
        |
        +--> 嵌入式播放器 WebView
                  |
                  v
            聚焦播放桥 --> myguo Go 服务
        |
        +--> SQLite user data
        +--> MediaServer and FFmpeg
```

## 开发

```bash
go mod tidy
go test ./...
CGO_ENABLED=0 go build -o /dev/null .
node frontend/ui-regression.test.js
go test -race ./internal/uiapp
node --test e2e/*.test.cjs
```

修改界面时，保持列表页使用响应式卡片网格；图标使用内联 SVG，不要使用 Emoji 或位图符号；保留标题栏和侧边栏的原生窗口拖拽行为。

## 许可

本项目仅供个人技术研究与本地使用。短剧内容和元数据版权归原权利人所有。

重新分发前请阅读 `THIRD_PARTY.md`。发布包内置启用 GPL 组件的 FFmpeg，相关二进制需要遵守适用的 GPL 义务。
