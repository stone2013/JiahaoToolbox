# 官方宣传片集成与发布准备

## 成片确认

选择已验收正式片 JiahaoToolbox_Promo_Final/JiahaoToolbox_Promo_Final_1080p60.mp4，61,749,714字节、90.000秒、5400帧、1920×1080、60/1 FPS、H.264/AAC48kHz立体声。

源片SHA256：5977fa94a696262b57eb2ce6c1ac2999b093479b8922aab3c1024bbde039f042。工作区两份同名原片哈希一致，并与原成片QA记录一致。没有选择61秒旧片、10秒测试片或SMV6独立3秒片头。

网页播放版已存在于 website/public/media/jiahao-toolbox-promo.mp4，20,785,577字节；SHA256为376c40a129adb7d05c9f5bbc9c740d2b565b302b7f15296be05df4137cf083d5。它来自该正式片的既有FFmpeg压缩：H.264 CRF24 / AAC128k 48kHz，原生1920×1080、60/1 FPS，5400帧。容器90.005秒为音频编码尾部时间差。

本次通过ffprobe、MP4 box分析和FFmpeg完整解码复核兼容性。moov位于mdat之前，faststart有效，音轨非静音；无需再次转码或放大。原始成片未修改。复用已存在的视频路径，不重复存储20.8MB文件，不删除旧文件或破坏已发布视频链接。

## 页面改动

- 标题：官方宣传片。
- 副标题：SMV6 出品 · 嘉豪工具箱功能展示与实际演示。
- 视频下方增加“下载嘉豪工具箱”，固定跳转https://github.com/stone2013/JiahaoToolbox/releases。
- 修复video HTML固定高度导致的黑边问题；CSS height:auto，随宽度保持16:9。
- 保留原生controls、高清poster、移动端布局、preload=none与非自动播放。
- 同时捕捉video和source加载失败，显示明确提示及下载片源入口。

## 部署与限制

沿用 /JiahaoToolbox/ 基路径与现有 .github/workflows/pages.yml。工作流无需修改；桌面源码和Release不变。预计播放地址：https://stone2013.github.io/JiahaoToolbox/media/jiahao-toolbox-promo.mp4 。此片源在上一次官网部署中已经公开，本次改动是待确认的栏目和播放器更新。

检查时站点构建约22.0MB，视频约20.8MB。根据GitHub官方文档，普通Git单文件超过100MiB被阻止，已发布Pages站点不大于1GB，Pages月流量软限制100GB。当前单片体积适合这一静态站点；若大规模观看，以20.8MB整片粗算约4800次完整传输即可接近100GB，实际包含缓存、请求范围及其他资源，不能当作允许播放次数。流量增大时应另行确认将片源改为GitHub Release资产或可靠视频托管。

官方依据：
- https://docs.github.com/en/pages/getting-started-with-github-pages/github-pages-limits
- https://docs.github.com/en/repositories/working-with-files/managing-large-files/about-large-files-on-github

## 真实性与隐私

沿用已验收成片的真实Windows录屏、已有隐私遮挡和原创音轨；未改变监控数据、诊断结果或增加未实现功能。原成片独立审查及捕获追溯保留在宣传片工程。网站本次改动不加入虚构演示、追踪服务或第三方视频。

文件音轨及默认非静音状态可自动验证；未新增扬声器或耳机完整试听，也未进行真实iOS/Safari实机测试。

## 当前授权边界

本轮仅本地修改、构建、测试和提交。远程推送、合并main或新的公开部署必须等待用户明确确认。本次未修改GitHub Pages设置。现有官网的已上线状态不表示本次修改已发布。
