# myguo E2E 自动化测试用例

基于 MyGo `plugins/automation`（Unix socket JSON 协议）驱动已编译 App。

前置条件：`go build -tags e2e` 编译，`MYGO_AUTOMATION=/tmp/myguo-test.sock` 启动。

## 模块一：启动与基础框架（A01–A08）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| A01 | App 窗口存在 | `windows` | 列出窗口 | 至少一个窗口，kind=page |
| A02 | 窗口尺寸正确 | `windows` | 读取 contentBounds | width >= 1000, height >= 640 |
| A03 | 页面加载完成 | `eval` | 读取 `document.readyState` | `complete` |
| A04 | 侧边栏存在 | `eval` | `#sidebar` 存在 | 非 null |
| A05 | 侧边栏导航项 >= 8 | `eval` | `.nav-item` 数量 | >= 8 |
| A06 | 主内容区域存在 | `eval` | `.main-content` 存在 | 非 null |
| A07 | Toast 容器存在 | `eval` | `#toast` 存在 | 非 null |
| A08 | 视图容器总数 >= 11 | `eval` | `.view-page` 数量 | >= 11 |
| A09 | 前端脚本已加载 | `eval` | 检查 `window.mygo.call`、`switchView`、`apiCall` 类型 | 全部可用 |

## 模块二：视图路由切换（B01–B12）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| B01 | 推荐视图默认激活 | `eval` | 检查 `#view-recommend.active` | `true` |
| B02 | 切换到找剧 | `eval` | `switchView('discover')` + 检查 | `#view-discover.active` |
| B03 | 切换到排行榜 | `eval` | `switchView('rank')` + 检查 | `#view-rank.active` |
| B04 | 切换到新剧 | `eval` | `switchView('new')` + 检查 | `#view-new.active` |
| B05 | 切换到历史 | `eval` | `switchView('history')` + 检查 | `#view-history.active` |
| B06 | 切换到收藏 | `eval` | `switchView('favorites')` + 检查 | `#view-favorites.active` |
| B07 | 切换到预约 | `eval` | `switchView('reservations')` + 检查 | `#view-reservations.active` |
| B08 | 切换到设置 | `eval` | `switchView('settings')` + 检查 | `#view-settings.active` |
| B09 | 切换回推荐 | `eval` | `switchView('recommend')` + 检查 | `#view-recommend.active` |
| B10 | 同一视图切换幂等 | `eval` | 连续 `switchView('discover')` 两次 | 不报错，仍 active |
| B11 | 侧边栏导航项同步高亮 | `eval` | 切到 discover 后 `.nav-item.active` 的 data-view | 匹配 |
| B12 | 无效视图不崩溃 | `eval` | `switchView('nonexistent')` | 不抛出异常 |

## 模块三：推荐视图 Feed（C01–C08）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| C01 | Feed 视频池数量 >= 3 | `eval` | `.feed-video` 数量 | >= 3 |
| C02 | Feed 视频有 playsinline | `eval` | `#feed-video-0.playsInline` | `true` |
| C03 | Feed 视频有 portrait class | `eval` | `#feed-video-0.className` 含 portrait 或 landscape | true |
| C04 | Feed 三个视频元素存在 | `eval` | `#feed-video-0/1/2` 均非 null | true |
| C05 | Feed hint toast 存在 | `eval` | `#feed-hint-toast` | 非 null |
| C06 | IPC GetHotSearch 可调 | `eval` | `window.mygo.call('GetHotSearch')` | 不抛异常 |
| C07 | IPC GetHotSearch 返回数组 | `eval` | 结果 | Array 且 length > 0 |
| C08 | IPC SetFeedScrollCapture 可调 | `eval` | `window.mygo.call('SetFeedScrollCapture', true)` | 不抛异常 |

## 模块四：找剧视图 Discover（D01–D10）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| D01 | 搜索输入框存在 | `eval` | `#search-input` | 非 null |
| D02 | 体裁 tab >= 3 | `eval` | `.genre-chip` 数量 | >= 3 |
| D03 | 默认体裁是真人剧 | `eval` | `.genre-chip.active` 的 data-genre | `short_play` |
| D04 | 筛选面板存在 | `eval` | `#discover-filters` | 非 null |
| D05 | 子筛选容器存在 | `eval` | `#discover-subfilters` | 非 null |
| D06 | Discover grid 容器存在 | `eval` | `#discover-grid` | 非 null |
| D07 | 加载更多按钮存在 | `eval` | `#btn-discover-load-more` | 非 null |
| D08 | Discover 加载后有卡片 | `eval` | 等待后 `#discover-grid .drama-card` 数量 | > 0 |
| D09 | IPC GetFilterPanel 可调 | `eval` | `window.mygo.call('GetFilterPanel','short_play')` | 不抛异常 |
| D10 | Drama card 含封面图 | `eval` | 第一张 `.drama-card img` 存在 | 非 null |

## 模块五：排行榜 Rank（E01–E05）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| E01 | 排行榜内容 tab 存在 | `eval` | `#rank-content-tabs .rank-content-tab` 数量 | >= 1 |
| E02 | 排行榜列表容器存在 | `eval` | `#rank-items-list` | 非 null |
| E03 | 排行榜 board tab 存在 | `eval` | `#rank-board-tabs` | 非 null |
| E04 | IPC GetRankings 可调 | `eval` | `window.mygo.call('GetRankings')` | 不抛异常 |
| E05 | 排行榜加载后有列表项 | `eval` | 等待后 `#rank-items-list` 子节点 | > 0 |

## 模块六：新剧 New Releases（F01–F05）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| F01 | 新剧主 tab >= 2 | `eval` | `#new-main-tabs .rank-content-tab` 数量 | >= 2 |
| F02 | 默认 tab 是热播新剧 | `eval` | `.rank-content-tab.active` 的 data-tab | `hot` |
| F03 | 日历日期容器存在 | `eval` | `#new-calendar-dates` | 非 null |
| F04 | 新剧 grid 容器存在 | `eval` | `#new-drama-grid` | 非 null |
| F05 | 新剧加载后有卡片 | `eval` | 等待后 `#new-drama-grid .drama-card` | > 0 |

## 模块七：历史 History（G01–G06）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| G01 | 历史 grid 容器存在 | `eval` | `#history-grid` | 非 null |
| G02 | 历史筛选 pills >= 3 | `eval` | `#history-filter-pills .filter-chip` 数量 | >= 3 |
| G03 | 历史搜索框存在 | `eval` | `#history-search-input` | 非 null |
| G04 | 清空历史按钮存在 | `eval` | `#btn-clear-all-history` | 非 null |
| G05 | IPC GetHistory 可调 | `eval` | `window.mygo.call('GetHistory')` | 不抛异常 |
| G06 | IPC GetHistory 返回数组 | `eval` | 结果 | Array |

## 模块八：收藏与预约（H01–H08）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| H01 | 收藏 grid 容器存在 | `eval` | `#favorites-grid` 或 `#view-favorites .drama-grid` | 非 null |
| H02 | IPC GetFavorites 可调 | `eval` | `window.mygo.call('GetFavorites')` | 不抛异常 |
| H03 | IPC GetFavorites 返回数组 | `eval` | 结果 | Array |
| H04 | 预约 grid 容器存在 | `eval` | `#view-reservations .drama-grid` | 非 null |
| H05 | IPC GetReservations 可调 | `eval` | `window.mygo.call('GetReservations')` | 不抛异常 |
| H06 | IPC GetReservations 返回数组 | `eval` | 结果 | Array |
| H07 | IPC GetLikes 可调 | `eval` | `window.mygo.call('GetLikes')` | 不抛异常 |
| H08 | IPC IsFavorite 可调 | `eval` | `window.mygo.call('IsFavorite','999')` | 不抛异常 |

## 模块九：搜索功能（I01–I08）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| I01 | 搜索建议容器存在 | `eval` | `#search-suggestions` | 非 null |
| I02 | 清空搜索按钮存在 | `eval` | `#btn-clear-search` | 非 null |
| I03 | IPC GetSuggestions 可调 | `eval` | `window.mygo.call('GetSuggestions','总裁')` | 不抛异常 |
| I04 | IPC GetSuggestions 返回数组 | `eval` | 结果 | Array |
| I05 | IPC Search 可调 | `eval` | `window.mygo.call('Search','总裁')` | 不抛异常 |
| I06 | 搜索结果视图存在 | `eval` | `#view-search-results` | 非 null |
| I07 | 搜索结果 header 存在 | `eval` | `#search-query-header` | 非 null |
| I08 | 搜索结果 grid 存在 | `eval` | `#search-grid` | 非 null |

## 模块十：设置与系统（J01–J10）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| J01 | 设置页标题存在 | `eval` | `.settings-page-title` 文本 | 非空 |
| J02 | 头像元素存在 | `eval` | `#settings-avatar` | 非 null |
| J03 | 昵称元素存在 | `eval` | `#settings-nickname` | 非 null |
| J04 | 登录 badge 存在 | `eval` | `#settings-login-badge` | 非 null |
| J05 | IPC GetCacheSize 可调 | `eval` | `window.mygo.call('GetCacheSize')` | 返回 number |
| J06 | IPC GetDeviceInfo 可调 | `eval` | `window.mygo.call('GetDeviceInfo')` | 返回对象含 deviceId |
| J07 | IPC GetProfile 可调 | `eval` | `window.mygo.call('GetProfile')` | 返回对象 |
| J08 | 清除缓存按钮存在 | `eval` | `#btn-settings-clear-cache` | 非 null |
| J09 | 设备 ID 元素存在 | `eval` | `#settings-device-id` | 非 null |
| J10 | 缓存大小元素存在 | `eval` | `#settings-cache-size` | 非 null |

## 模块十一：详情页与播放器（K01–K10）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| K01 | 详情视图容器存在 | `eval` | `#view-detail` | 非 null |
| K02 | 详情标题元素存在 | `eval` | `#detail-title` | 非 null |
| K03 | 集数 grid 存在 | `eval` | `#detail-episode-grid` | 非 null |
| K04 | 集数计数存在 | `eval` | `#detail-episode-count` | 非 null |
| K05 | 相关推荐区域存在 | `eval` | `#detail-related-section` | 非 null |
| K06 | 演员区域存在 | `eval` | `#detail-actors-section` | 非 null |
| K07 | 主播放器 video 存在 | `eval` | `#main-video` | 非 null |
| K08 | 主播放器有 playsinline | `eval` | `#main-video.playsInline` | `true` |
| K09 | 集数展开按钮存在 | `eval` | `#detail-episode-toggle` | 非 null |
| K10 | 演员视图存在 | `eval` | `#view-actor` | 非 null |

## 模块十二：IPC 桥接完整性（L01–L10）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| L01 | window.mygo 存在 | `eval` | typeof | object |
| L02 | window.mygo.call 存在 | `eval` | typeof | function |
| L03 | IPC 不存在方法返回 reject | `eval` | 调用不存在方法 catch | 被捕获 |
| L04 | IPC ToggleFavorite 拒绝无效 ID | `eval` | `window.mygo.call('ToggleFavorite','999')` | Promise reject，且不写入 |
| L05 | IPC ToggleLike 拒绝无效 ID | `eval` | `window.mygo.call('ToggleLike','999','999')` | Promise reject，且不写入 |
| L06 | IPC ClearNowPlaying 可调 | `eval` | `window.mygo.call('ClearNowPlaying')` | 不崩溃 |
| L07 | IPC SetPlaybackKeepAwake 可调 | `eval` | `window.mygo.call('SetPlaybackKeepAwake',false)` | 不崩溃 |
| L08 | IPC SendNativeNotification 可调 | `eval` | `window.mygo.call('SendNativeNotification','e2e','test')` | 不崩溃 |
| L09 | IPC ShowConfirmDialog 可调 | `eval` | `window.mygo.call('ShowConfirmDialog','e2e')` | 不崩溃 |
| L10 | IPC SaveProgress 拒绝无效 ID | `eval` | `window.mygo.call('SaveProgress','999',...)` | Promise reject，且不写入 |

## 模块十三：安全与规范（M01–M05）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| M01 | 页面无 emoji | `eval` | 扫描 body.innerHTML emoji Unicode | 无匹配 |
| M02 | handleImgError 全局可用 | `eval` | typeof | function |
| M03 | escapeHtml 全局可用 | `eval` | typeof | function |
| M04 | 无 javascript: onclick | `eval` | `document.querySelector('[onclick^="javascript:"]')` | null |
| M05 | 图片 onerror 已绑定 | `eval` | `.drama-card img[onerror]` 存在（如有卡片） | true 或无卡片 |

## 模块十四：视觉回归截图（N01–N05）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| N01 | 推荐视图截图 | `capture` | 切到 recommend 后截图 | PNG 尺寸 > 0 |
| N02 | 找剧视图截图 | `capture` | 切到 discover 后截图 | PNG 尺寸 > 0 |
| N03 | 排行榜视图截图 | `capture` | 切到 rank 后截图 | PNG 尺寸 > 0 |
| N04 | 设置视图截图 | `capture` | 切到 settings 后截图 | PNG 尺寸 > 0 |
| N05 | 历史视图截图 | `capture` | 切到 history 后截图 | PNG 尺寸 > 0 |

## 模块十五：稳定与并发（P01–P04）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| P01 | Idle 稳定 | `idle` | 等待页面 settle | 无 timeout |
| P02 | 连续切换 10 个视图 | `eval` | 循环 switchView | 不崩溃 |
| P03 | App 进程仍响应 | `windows` | 所有测试后列窗口 | 正常返回 |
| P04 | 总截图数 >= 5 | `capture` | 各视图截图汇总 | >= 5 |

---

以下为完整覆盖拓展模块。

## 模块十六：剩余 IPC 方法完整覆盖（Q01–Q24）

| # | 用例名 | 方法/IPC | 步骤 | 通过条件 |
|---|--------|----------|------|----------|
| Q01 | GetRecommendFeed 可调 | IPC | 调用 | 不抛异常 |
| Q02 | GetRecommendFeed 返回对象/数组 | IPC | 检查返回值类型 | 非空 |
| Q03 | GetDiscover 可调 | IPC | `GetDiscover` | 不抛异常 |
| Q04 | GetNewReleases 可调 | IPC | 调用 | 不抛异常 |
| Q05 | GetRecommend 可调 | IPC | 调用 | 不抛异常 |
| Q06 | GetSeriesDetail 用有效 ID | IPC | 从 Discover grid 取第一个卡片 ID | 不抛异常 |
| Q07 | GetSeriesDetail 返回标题 | IPC | 检查返回值 title 字段 | 非空字符串 |
| Q08 | GetSeriesDetail 无效 ID 返回错误 | IPC | 传入 `"999"` | Promise reject |
| Q09 | GetActorProfile 可调 | IPC | 调用 `"test"` | 不抛异常 |
| Q10 | GetActorWorks 可调 | IPC | 调用 `"test"` | 不抛异常 |
| Q11 | CheckExistingAccount 可调 | IPC | 调用 | 不抛异常 |
| Q12 | ImportHgplayerAccount 可调 | IPC | 调用（可能 reject） | 被捕获 |
| Q13 | LoginWithCookies 空 Cookie 返回错误 | IPC | 传空字符串 | 被捕获（reject） |
| Q14 | Logout 可调 | IPC | 调用 | 不崩溃 |
| Q15 | RefreshUserProfile 可调 | IPC | 调用 | 不崩溃 |
| Q16 | ToggleReservation 可调 | IPC | 传 `"999"` | 被捕获 |
| Q17 | TogglePictureInPicture 可调 | IPC | 调用 | 不崩溃 |
| Q18 | UpdateNowPlaying 可调 | IPC | 传测试数据 | 不崩溃 |
| Q19 | WarmupFeed 可调 | IPC | 调用 | 不崩溃 |
| Q20 | PlayEpisode 无效 ID 返回错误 | IPC | 传 `"999"` | 被捕获（不 panic） |
| Q21 | DeleteHistory 已移除 | IPC | 调用 | 拒绝调用 |
| Q22 | ClearHistory 已移除 | IPC | 调用 | 拒绝调用 |
| Q23 | ClearCache 可调 | IPC | 调用 | 返回 number |
| Q24 | ResetDevice 可调 | IPC | 调用（重建设备指纹） | 不崩溃 |

## 模块十七：详情页完整流程（R01–R14）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| R01 | Discover 有可点击卡片 | 检查 `#discover-grid .drama-card` 数量 | > 0 |
| R02 | 点击第一个卡片打开详情 | 调用 `openSeriesDetail(firstId)` | 不抛异常 |
| R03 | 详情视图激活 | 检查 `#view-detail.active` | true |
| R04 | 详情标题已填充 | `#detail-title` textContent 非空 | 非空且非 "--" |
| R05 | 详情封面图存在 | `#detail-poster img` 或 `#detail-poster` | 存在 |
| R06 | 详情播放按钮存在 | `#btn-detail-play` | 非 null |
| R07 | 详情收藏按钮存在 | `#btn-detail-favorite` | 非 null |
| R08 | 详情标签区域存在 | `#detail-tags` | 非 null |
| R09 | 详情描述区域存在 | `#detail-description` | 非 null |
| R10 | 集数列表已填充 | `#detail-episode-grid` 子节点 > 0（等待后） | > 0 |
| R11 | 返回按钮存在 | `#btn-back-from-detail` | 非 null |
| R12 | 点击返回按钮回到上一个视图 | 点击后检查 active 视图 | 非详情 |
| R13 | 详情热度字段存在 | `#detail-heat` | 非 null |
| R14 | 详情状态字段存在 | `#detail-status` | 非 null |

## 模块十八：搜索完整流程（S01–S12）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| S01 | 在搜索框输入文本 | `searchInput.value = '总裁'; dispatchEvent('input')` | 值已设置 |
| S02 | 输入后清空按钮显示 | 检查 `#btn-clear-search` display | 非 none |
| S03 | 按 Enter 键触发搜索 | dispatchEvent keydown Enter | 搜索结果视图激活 |
| S04 | 搜索结果 header 显示查询 | `#search-query-header` 包含 "总裁" | 包含 |
| S05 | 搜索结果 grid 有 loader 或结果 | `#search-grid` 非空 | 非空 |
| S06 | 等待搜索结果加载 | 等待 3 秒后检查 | `.drama-card` > 0 或无结果提示 |
| S07 | 点击清空按钮清空输入 | `btn-clear-search.click()` | input.value = "" |
| S08 | 清空后按钮隐藏 | `#btn-clear-search` display | none |
| S09 | 输入特殊字符不崩溃 | 输入 `<script>alert(1)</script>` | 不抛异常 |
| S10 | 输入超长字符串不崩溃 | 输入 500 字符 | 不抛异常 |
| S11 | 输入空白字符串触发搜索 | 输入空格后 Enter | 无异常（可能忽略） |
| S12 | 搜索建议在输入后显示或隐藏 | 输入 "总裁" 后检查 suggestions | display 被 JS 控制 |

## 模块十九：历史与收藏交互（T01–T10）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| T01 | 点击 "未看完" pill | `.filter-chip[data-filter='unwatched']` click | active 切换 |
| T02 | 点击 "已看完" pill | `.filter-chip[data-filter='completed']` click | active 切换 |
| T03 | 点击 "全部" pill 切回 | `.filter-chip[data-filter='all']` click | active 切换 |
| T04 | 历史搜索框可输入 | `historySearchInput.value = 'test'` | 值已设置 |
| T05 | 历史搜索框清空 | 值设为 "" | 无异常 |
| T06 | 收藏 tab 切换（如有 tab） | 检查收藏视图 tab 并点击 | 不崩溃 |
| T07 | 清空历史按钮可点击 | 按钮非 disabled | true |
| T08 | 点击收藏卡片打开详情（如有） | 第一张卡片 click | 不崩溃 |
| T09 | 收藏视图标题存在 | 收藏区域 heading | 非 null |
| T10 | 预约视图标题存在 | 预约区域 heading | 非 null |

## 模块二十：设置交互（U01–U10）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| U01 | 点击 "立即登录" 显示登录面板 | `btn-show-login.click()` | 面板 display 变化 |
| U02 | 登录面板 textarea 存在 | `#login-cookie-input` | 非 null |
| U03 | 登录面板 token input 存在 | `#login-token-input` | 非 null |
| U04 | 粘贴剪贴板按钮存在 | `#btn-paste-clipboard` | 非 null |
| U05 | 取消登录按钮存在 | `#btn-cancel-login` | 非 null |
| U06 | 提交登录按钮存在 | `#btn-submit-login` | 非 null |
| U07 | 点击取消登录关闭面板 | `btn-cancel-login.click()` | 面板隐藏 |
| U08 | 重置设备按钮存在 | `#btn-settings-reset-device` | 非 null |
| U09 | 刷新资料按钮存在 | `#btn-refresh-profile` | 非 null |
| U10 | 状态指示器存在 | `#settings-status-indicator` | 非 null |

## 模块二十一：播放器 UI（V01–V08）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| V01 | 播放器关闭按钮存在 | `#btn-close-player` | 非 null |
| V02 | 视频容器存在 | `#video-container` | 非 null |
| V03 | 视频 loader 存在 | `#video-loader` | 非 null |
| V04 | 中心播放 badge 存在 | `#center-play-badge` | 非 null |
| V05 | 环境光 canvas 存在 | `#modal-ambient-canvas` | 非 null |
| V06 | 评论面板 overlay 存在 | `#comment-sheet-overlay` | 非 null |
| V07 | 评论关闭按钮存在 | `#btn-close-comment-sheet` | 非 null |
| V08 | 评论列表容器存在 | `#comment-list` | 非 null |

## 模块二十二：发现页交互（W01–W06）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| W01 | 点击漫剧 tab | genre chip data-val=comic_series click | active 切换 |
| W02 | 点击 AI 剧 tab | genre chip data-val=ai_series click | active 切换 |
| W03 | 点击真人剧 tab 切回 | genre chip data-val=short_play click | active 切换 |
| W04 | 筛选 chip 有 data-key 属性 | `.filter-chip` 至少一个有 data-key | true |
| W05 | 筛选 chip 有 data-val 属性 | `.filter-chip` 至少一个有 data-val | true |
| W06 | 加载更多按钮可点击 | `btn-discover-load-more.click()` | 不崩溃 |

## 模块二十三：排行与新剧交互（X01–X06）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| X01 | 排行榜有多个 content tab | `.rank-content-tab` 数量 | >= 2 |
| X02 | 点击第二个排行 tab | click 后 active 切换 | true |
| X03 | 点击第一个排行 tab 切回 | click 后 active 切回 | true |
| X04 | 新剧点击日历 tab | `data-tab='calendar'` click | active 切换 |
| X05 | 日历 tab 显示日期列表 | `#new-calendar-dates` 子节点 | 可能 >= 0 |
| X06 | 切回热播 tab | `data-tab='hot'` click | active 切换 |

## 模块二十四：边界条件（Y01–Y10）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| Y01 | IPC 传 null 参数 | `GetSeriesDetail(null)` | 被捕获 |
| Y02 | IPC 传 undefined 参数 | `GetSeriesDetail(undefined)` | 被捕获 |
| Y03 | IPC 传超长字符串 | `Search('a'.repeat(5000))` | 被捕获 |
| Y04 | IPC 传负数 | `SaveProgress(-1,...)` | 被捕获 |
| Y05 | IPC 传对象而非字符串 | `GetSeriesDetail({})` | 被捕获 |
| Y06 | 并发 5 个 IPC 调用 | Promise.all 5 个 GetSuggestions | 全部完成 |
| Y07 | 并发 10 个视图切换 | Promise.all + switchView | 不崩溃 |
| Y08 | 快速点击 genre chip | 连续 click 三个 chip | 不崩溃 |
| Y09 | 空视图切换后 eval | switchView 后 eval | 正常返回 |
| Y10 | CSS 变量存在 | `--bg-primary` 在 :root | 非空 |

## 模块二十五：无障碍（Z01–Z08）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| Z01 | 所有 img 有 alt 或 aria-hidden | 检查 `img:not([alt]):not([aria-hidden])` | 0 个 |
| Z02 | nav 按钮有文本或 aria-label | `.nav-item` 的 textContent 非空 | true |
| Z03 | 搜索输入有 placeholder | `#search-input.placeholder` 非空 | true |
| Z04 | 历史搜索有 placeholder | `#history-search-input.placeholder` 非空 | true |
| Z05 | video 有 aria-label 或 title | `#main-video` | true 或无障碍策略允许 |
| Z06 | sidebar resizer 有 role | `#sidebar-resizer[role]` | true |
| Z07 | loader 有 role=status | `.macos-loader-hud` 或 `.video-loader[role]` | true |
| Z08 | html lang 属性 | `document.documentElement.lang` 非空 | true |

## 模块二十六：性能计时（AA01–AA06）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| AA01 | DOM 就绪时间 < 10 秒 | `performance.timing.domContentLoadedEventEnd - navigationStart` | < 10000ms |
| AA02 | 切换视图 < 200ms | performance.now 包裹 switchView | < 200ms |
| AA03 | GetHotSearch 响应 < 10 秒 | performance.now 包裹 IPC | < 10000ms |
| AA04 | GetHistory 响应 < 5 秒 | performance.now 包裹 IPC | < 5000ms |
| AA05 | GetCacheSize 响应 < 5 秒 | performance.now 包裹 IPC | < 5000ms |
| AA06 | GetSuggestions 响应 < 10 秒 | performance.now 包裹 IPC | < 10000ms |

## 模块二十七：推荐流播放调度（BB01–BB07）

| # | 用例名 | 方法 | 步骤 | 通过条件 |
|---|--------|------|------|----------|
| BB01 | 下一部播放源完成准备 | 轮询 `feedSlots[feedSlotNext].video.src` | 最多 15 秒 | src 非空 |
| BB02 | 推荐流使用明确媒体路由 | 检查下一部 src | HLS 使用媒体 playlist；progressive/full cache 路由可明确分类 |
| BB03 | 首帧前不隐藏封面 | 检查 `feedVideoHasFirstFrame` 与 poster hidden | 两者状态一致 |
| BB04 | 推荐流切换保持干净 | 调用 `switchFeedDrama(1)` 并等待首帧数据 | index 前进、无准备提示、previous 未 priming、视频未暂停 |
| BB05 | 下一部自动播放稳定 | 切换后等待 1 秒检查当前视频 | 未暂停、userPaused=false、src 存在 |
| BB06 | 推荐分页持续补足 | 轮询 items、loading 与 retry 状态 | 已有 4 项后续，或分页正在加载/重试 |
| BB07 | 连续下滑保持连续播放 | 连续 3 次下滑并观察 UI | 索引精确前进 3 次；过程无 hint/loader，最终动画复位且视频继续播放 |

## 模块二十八：真实账号同步（AC01–AC08）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| AC01 | 匿名数据先行 | 匿名浏览推荐并播放一集 | 本地数据只包含匿名会话真实产生的数据 |
| AC02 | 登录后拉取账号数据 | 粘贴有效 Cookie 登录并进入历史/收藏/点赞/预约 | 四个列表返回登录账号真实数据，无前一个匿名会话残留 |
| AC03 | 收藏断网排队 | 断网后切换收藏，再恢复网络 | pending 操作按最新意图同步，服务端状态与本地一致 |
| AC04 | 历史断网排队 | 断网后保存进度/删除历史，再恢复网络 | 上传与删除顺序正确，远端进度与本地一致 |
| AC05 | 点赞断网排队 | 断网后点赞/取消点赞，再恢复网络 | pending 队列清空，远端点赞状态正确 |
| AC06 | 预约切换通知 | 登录后预约未上线剧并等待远端列表同步 | 预约成功通知只出现一次，预约列表更新 |
| AC07 | 登出边界 | 同步完成后登出并重新进入四个列表 | 无登录态接口被调用；本地数据语义保持不变 |
| AC08 | 切换账号边界 | 登出后登录另一账号 | 内存缓存失效，四个列表不显示上一个账号数据 |

## 模块二十九：受保护播放（AD01–AD08）

| # | 用例名 | 步骤 | 通过条件 |
|---|--------|------|----------|
| AD01 | 受保护 progressive 播放 | 登录后播放受保护剧并检查 `feedSlots` 当前 src | 起播成功，首帧出现，无 decoder error |
| AD02 | 受保护 HLS 播放 | prefer HLS 路径播放同一内容 | media playlist 加载成功，片段可解密播放 |
| AD03 | 画质降级 | 模拟首选画质上游失败或选择低于源画质 | MediaServer 回退到可播画质 |
| AD04 | Full cache 起播 | 首次完整缓存后重启并离线播放 | SQLite 索引命中的物理文件存在，可离线起播 |
| AD05 | 账号切换清协商缓存 | 匿名 resolve 后登录/切换账号 | 播放协议缓存清空，重新 resolve 后成功 |
| AD06 | 快速滑动调度 | 连续下滑 5 个推荐卡 | 当前流不被 prefetch replacement 取消；相邻预取窗口刷新 |
| AD07 | 缓存驱逐不中断 | 预热多个分集直至触发 2GB 清理 | 正在播放文件/active HLS 目录不被删除 |
| AD08 | Clear Cache 后恢复 | 播放中或播放后点击清除缓存 | 物理文件与 SQLite media_cache 索引同步清理，下次播放重新 resolve |
