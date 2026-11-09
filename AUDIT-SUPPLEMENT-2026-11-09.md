# hongguo-mygo 补充审查报告 — 2026-11-09

审查对象：`main @ 93f91024acbc12e1431afe8e0a3f35cdee55dad9`。本报告只补查主报告 `AUDIT-2026-11-09.md` 未完成的评论区、弹幕、播放器 WebView、推荐流视觉与详情宽屏布局；只读代码、历史提交与 `/tmp/myguo-audit-screens/detail-ready-1600.png` 夹具截图，不改产品代码、不提交。

## 1. 评论区

**总结结论：存在 P1 阻断级样式缺失。打开/关闭、分页、空态、错误态、回复分页、长文本转义与计数同步逻辑完整，但 `d42c2b1` 把评论抽屉的全部定位、滚动和排版样式删除，当前 DOM 会按普通文档流堆叠，评论功能实际不可用。**

### B-001 评论抽屉样式被整体删除

- **严重度**：P1。
- **所在页面**：推荐流播放器评论抽屉、剧集播放器评论抽屉。
- **文件:行号**：`frontend/index.html:298`、`frontend/app.js:3615`、`frontend/app.js:3694`、`frontend/app.js:3951`、`frontend/styles.css:33`、`frontend/styles.css:1398`；删除来源：`d42c2b1` 对 `frontend/styles.css` 的 `@@ -890,823 +889,0 @@` hunk；可用旧样式见 `6f56f0f:frontend/styles.css:1304-1559`。
- **问题描述**：评论 overlay、sheet、header、list、item、回复、错误/空态和 `comments-open` 布局在当前三个样式表中没有任何规则；仅剩 `--comment-sheet-width: 420px` 和 `.comment-content` 的可选中文本规则。样式删除后，`position: fixed` 的遮罩、420px 右侧抽屉、列表滚动、36px 头像网格、meta/content/stats 排版全部消失。
- **判定依据**：`initCommentSheet()`、`openCommentSheet()`、`renderCommentSheet()` 和 `closeCommentSheet()` 仍在维护 loading/error/分页 token、IntersectionObserver、回复游标、Esc 关闭、`textContent` 转义和计数回写；但 `git show 93f9102:frontend/styles.css` 中没有 `.comment-sheet-overlay`、`.comment-sheet`、`.comment-list`、`.comment-item` 或 `body.comments-open`。`d42c2b1` 的 diff 显示这些规则随 823 行样式被一并删除，而 `6f56f0f` 仍包含完整旧实现。
- **建议修法**：从 `6f56f0f:frontend/styles.css:1304-1559` 恢复评论样式块，并按当前共享播放器调整选择器：删除 `#player-modal`、`.video-container` 等旧 DOM 目标；保留/改写 `body.comments-open .feed-stage { right: var(--comment-sheet-width) }` 为当前 stage 的宽度让位方案；移动端与 1600px 宽度分别验证 overlay 覆盖、列表滚动、长文本换行和播放画面让位。
- **修复影响面 / 风险**：影响 feed 与剧集播放器的评论入口和抽屉视觉。主要风险是恢复旧选择器时误复活已删除的 player modal；应同时补一条静态 CSS/DOM 断言，确保每个评论类名都有布局规则。

评论区其余路径已查：打开校验 `seriesId/episodeId`，请求 token 防竞态；列表通过 `hasMore/nextCursor` 与 sentinel 分页；错误态和空态文案存在；`textContent` 防特殊字符注入；`applyCommentPage()` 会同步当前集评论数；Esc、按钮和遮罩点击均可关闭。这些逻辑本身无问题，但当前被 B-001 的样式缺失阻断。

## 2. 弹幕

**总结结论：已查、无问题。开关、持久化、轨道分配、碰撞检测、暂停同步、seek 重置、设置生效、发送失败回滚和关闭后停止逻辑完整；大量弹幕通过每时间窗最多 launch 20 条与无可用轨道即丢弃来限流。**

判定依据：`frontend/danmaku.js:30-93` 读取/保存透明度、字号、字重、描边、行数；`setEnabled(false)` 递增请求 token、停止 rAF、清空 active 并隐藏层；`tick()` 在 800ms 回退或 3000ms 快进后清空重放；`selectLane()` / `willCollide()` 以容器宽度、已消费动画时间和 18px 间距判断重叠；`launchUntil()` 每帧最多 20 条；`pruneOld()` 只保留播放点后 120 秒数据；发送失败会从 items、seenDisplay 和 active 中移除本地弹幕。

## 3. 播放器 WebView 控件层

**总结结论：存在三个 P1 问题。控件 DOM、事件链路、清晰度/倍速/选集/PiP/全屏逻辑存在，且 feed 与剧集播放器复用 `mountPlayerChrome()`；但基础控制条样式也在 `d42c2b1` 被删除，进度条和图标菜单没有可用布局，环境光规则指向已删除的旧 modal 结构。另有两处未定义引用破坏系统上一/下一集与自动续播。**

### B-002 未定义 `swipeFeed` 与 `playerModal` 破坏媒体命令和自动续播

- **严重度**：P1。
- **所在页面**：播放器 WebView、macOS 系统媒体控制、视频 ended 自动续播。
- **文件:行号**：`frontend/app.js:1946`、`frontend/app.js:1950`、`frontend/app.js:4333`。
- **问题描述**：`handleMediaCommand('next'|'prev')` 调用不存在的 `swipeFeed()`；视频 `ended` 事件读取不存在的 `playerModal`。严格模式/模块环境中前者让媒体键上一/下一集直接抛错，后者让每一次 ended 处理抛 `ReferenceError`，后续自动下一集和下一季逻辑不会执行。
- **判定依据**：`git show 93f9102:frontend/app.js` 确认 `1946` 与 `1950` 调用 `swipeFeed()`，`4333` 读取 `playerModal`；同一版本全文均未声明二者。现有 `e2e/media_commands.test.cjs` 通过是因为测试夹具注入了 `swipeFeed` stub，掩盖了真实运行环境错误。
- **建议修法**：`next` / `prev` 改调当前方向的 `requestFeedSwitch(1, 'media-command')` / `requestFeedSwitch(-1, 'media-command')`；删除 `4333` 的 player modal 判断，前一行 `state.currentView !== 'recommend'` 已是有效守卫，或改为明确的 `feedSlots[feedSlotCurr]` 活跃槽校验。测试应删除注入 stub，并直接断言函数声明存在与真实命令路径。
- **修复影响面 / 风险**：影响系统媒体键、Now Playing 命令和剧集自动连播。需覆盖播放中、暂停、弹幕抽屉打开、最后一集切下一季和推荐流边界的命令竞争。

### B-003 播放器基础控制条样式同样被删除

- **严重度**：P1。
- **所在页面**：推荐流播放器、剧集播放器 WebView。
- **文件:行号**：`frontend/player-chrome.js:48-89`、`frontend/player-chrome.js:146-148`、`frontend/styles.css:605-618`、`frontend/styles.css:821-836`；删除来源：`d42c2b1` 同一 `@@ -890,823 +889,0 @@` hunk；可用旧样式见 `6f56f0f:frontend/styles.css:1038-1301`。
- **问题描述**：当前只有 `.feed-controls` 容器和 `.progress-time-tooltip` 的部分样式；`.progress-bar-wrap`、`.progress-bar-bg/-buffer/-current`、`.controls-row`、`.controls-left/right`、`.ctrl-btn`、`.time-display`、`.quality-selector`、`.speed-selector`、`.quality-menu/-opt`、`.speed-opt` 和 `.feed-fullscreen` 缺失。进度条没有宽高与填充，图标按钮没有尺寸，清晰度/倍速菜单不会成为绝对定位弹层，`feed-fullscreen` 状态类无布局效果。
- **判定依据**：`mountPlayerChrome()` 为 feed 与剧集播放器注入同一批 class/id，`initFeedControls()` 正确绑定播放、进度点击、速度、质量、选集、PiP 和全屏；但 `frontend/styles.css` 只在 `1391` 提供焦点样式，`frontend/liquid-controls.css` 只有 `.ctrl-btn.episodes-toggle`、`.ctrl-text-btn`、`.ep-btn` 和 speed menu 的底色，没有控制条骨架。`d42c2b1` diff 精确删除了上述旧规则。
- **建议修法**：从 `6f56f0f:frontend/styles.css:1038-1301` 恢复控制条基础样式，并把旧 `#player-*` 命名映射到当前 `#feed-*` 前缀；删除 `.player-overlay` / `.player-topbar` 等已不存在的 DOM。`.feed-container.feed-fullscreen` 需与 `schedulePlaybackVideoFit()` 和安全区变量一起重新设计，确保 1600px 全屏下控制条不贴边、视频按真实 fitted frame 缩放。
- **修复影响面 / 风险**：影响两套播放器的所有可见控件和窗口全屏。风险集中在旧 CSS 中 `#playlist-drawer`、`#player-modal` 等旧 ID 与当前前缀化 ID 不一致；恢复时应以 class 为基础，逐项映射前缀，并补 DOM/class 覆盖测试。

### 播放器其余控件结论

- **播放/暂停与缓冲**：已查，逻辑无独立问题。`toggleFeedPlay()` 处理无源恢复、暂停、用户意图和指示器；`waiting/playing` 更新 loader 与 `.is-buffering`，200ms 防闪显示加载，失败有自动回退和“视频加载失败，请重试”提示。当前可见效果受 B-003 影响。
- **进度条拖动/缓冲**：逻辑已查，无独立问题。点击按 wrap 矩形换算，tooltip 跟随鼠标；`durationchange/progress/seeking/timeupdate` 更新 buffer。但无基础样式时点击热区与可视进度不可用。
- **清晰度**：状态与菜单生成逻辑已查，无独立问题。`updateFeedQualityControls()` 使用播放信息中的可选清单，禁用项有样式意图；但 `.quality-menu` 无布局后实际呈现受 B-003 影响。
- **倍速**：菜单和 `playbackRate` 更新逻辑已查，无独立问题；换源时 `slot.video.playbackRate = state.feed.playbackSpeed || 1` 会恢复速度。可见菜单受 B-003 影响。
- **选集面板**：抽屉打开/关闭、当前集高亮、折叠展开、四列网格和返回顶部逻辑已查，无独立问题；`.feed-playlist-drawer` 样式仍存在。右侧 `feed-rail` 与左侧信息有固定 inset 和 bottom 分层，当前布局无新的独立问题。
- **上一/下一集按钮**：按钮点击路径正确，边界有 toast；但系统命令同名能力由 B-002 破坏。
- **PiP 与全屏**：PiP 调用共享 `togglePictureInPicture()`；全屏同时维护容器类与原生窗口切换。状态链路无独立缺陷，但 `feed-fullscreen` 样式缺失使容器态失效。
- **环境光**：控制器限制在播放态、页面可见时采样，使用 16×16 canvas 和约 60ms 间隔，hidden/pause/ended/seeking 会 deactivate；但当前 `body.player-ambient-glowing #player-stage` 与 `.video-container` 规则指向旧 modal 结构，新共享播放器没有对应 stage，视觉恢复逻辑实际失效。该项并入 B-003，建议恢复控制条时同步改挂到 `#feed-stage` 或当前视频容器。
- **两套入口一致性**：已查、无结构不一致。feed 与剧集播放器都调用 `mountPlayerChrome()`；差异仅来自各自状态绑定和 drawer 标题。实际视觉一致性必须先修复 B-003 后复验。
- **安全区**：现有 `--stage-chrome-inset` 只用于右栏默认值，控制条未读取；恢复样式时应为全屏和窗口动画明确 left/right/bottom inset，当前按缺失样式合并进 B-003。

## 4. 推荐流视觉细节

**总结结论：已查、无新的 P0/P1/P2 问题。**

判定依据：

- **右侧动作列**：`frontend/styles.css:344-413` 中 `.feed-rail` 固定右 `var(--stage-chrome-inset, 18px)`、bottom `84px`、纵向居中、间距 `16px`；图标容器 `44×44`，label 11px，active 分别使用红色/金色语义；`frontend/liquid-controls.css:175-190` 增强 hover、玻璃和阴影。结构与夹具截图一致。
- **计数格式**：`frontend/app.js:4072-4078` 的 `formatCount()` 对空值、万、亿做正确缩写，`评论/点赞` 使用同一格式；评论打开后通过 `updateFeedCommentCount()` 回写当前集。
- **标题/作者/简介区**：`frontend/styles.css:305-341` 使用 left 24px、bottom 74px、最大宽 `min(50%, 540px)`、两行 clamp；标题 18px、标签 11px、简介 12px，与右侧 rail 分层不重叠。作者字段由标题/标签/描述链路承载，未发现排版冲突。
- **频道切换栏**：`frontend/styles.css:186-229` 与 `frontend/liquid-controls.css:57-133` 的 active chip、滑块和玻璃层规则完整；`updateCatBarGlider()` 在 resize 后重算，位置与宽度使用 offsetLeft/offsetWidth。
- **安全区与窗口缩放**：feed stage 全屏、视频 portrait/landscape 双规则，`syncPlaybackVideoFit()` 在 resize、video resize 和容器 ResizeObserver 后按 `min(box.clientWidth/sourceWidth, box.clientHeight/sourceHeight)` 设置精确宽高；当前无可见的溢出或遮挡路径。若恢复控制条后改变 stage 让位，需在此处复测，但当前不构成独立问题。

## 5. 详情页宽屏布局核实

**总结结论：Y-001 成立，编号 B-004。根因不是内容区固定最大宽度，而是详情信息流始终单列；160px 固定海报加短文本 header 只占左上区域，选集作为后续块排在海报下方。与 A-008 的用户可见症状同属“少量固定元素 + 全宽父容器”，但不是同一个可删除的 `MaxWidth` 瓶颈。**

### B-004 详情页 1600px 宽度下右侧空白、选集布局不随宽度重构

- **严重度**：P1。
- **所在页面**：原生详情页宽屏布局。
- **文件:行号**：`internal/uiapp/view.go:893-895`、`internal/uiapp/view.go:1047`；对照 `/tmp/myguo-audit-screens/detail-ready-1600.png`。
- **问题描述**：1600×1000 截图中详情 hero 只占左侧约 40%，右侧大面积空白；选集以 52×34 小按钮和 8px 间距横向 wrap，但仍紧跟 hero 之后的纵向流，没有在宽屏下移到右侧栏或形成更大内容网格。
- **判定依据**：`renderDetail()` 在 `893` 创建普通 `ui.Row(detail-hero-row)`，`894` 海报固定 `160×220`，`895` metadata column 虽 `Grow(1)`，但短标题、标签、统计、描述和按钮不会横向铺满视觉重心；代码中详情路径没有 `MaxWidth` 限制，全仓库唯一 `MaxWidth` 在新剧卡片 `catalogCardSized()`。`1047` 的 `detail-episodes-wrap` 是全宽 `ui.Row(...).Wrap().Gap(8)`，因为它位于滚动容器中的 hero 之后，所以视觉上仍是海报下方的窄按钮流。截图与代码相互证实。
- **建议修法**：在详情滚动内容内引入宽屏布局分支：约 ≥1200px 时用两栏结构，左栏放海报 + metadata + actions + 原著/演员，右栏放选集和相关系列/改编；选集右栏可使用 3-4 列大按钮或虚拟网格，并跟随容器宽度扩展。若不想引入完整双栏，至少把 hero 内容居中并给 metadata/actions 设定可读最大宽度，同时让选集按钮在宽屏下横向展开到内容区而非只贴左。需在 `TestNativePageScreens` 或新增 1600×1000 渲染测试中断言选集区与 hero 的相对位置/宽度，而不是只断言模型列数。
- **修复影响面 / 风险**：影响详情页所有宽度的滚动与骨架一致性；要同步处理 A-012 指出的骨架缺少原著/演员占位，避免双栏重构放大加载态跳动。与 A-008 的关系：两者都表现为固定小卡/元素未占满宽父容器，但 A-008 是新剧卡片固定尺寸策略，B-004 是详情单列信息架构，不应通过简单删除 max width 解决。

## 6. 问题统计

| 严重度 | 数量 | 编号 |
| --- | ---: | --- |
| P0 | 0 | — |
| P1 | 4 | B-001、B-002、B-003、B-004 |
| P2 | 0 | — |

最终统计：**P0 0 / P1 4 / P2 0**。
