---
name: mygo-acrylic
description: 在 mygo（github.com/egoist/mygo）原生 UI 上做「亚克力」外观——系统磨砂材质（macOS NSVisualEffectView / Windows 11 Mica·Acrylic）+ 主题色薄纱窗口底 + 用户可调不透明度的工作卡 + 只用半透明洗色的控件。新建 mygo 应用的窗口外壳与组件、给卡片 / 侧栏写新控件、把现有界面改成亚克力兼容、或排查「某块实心底把磨砂抠穿了」时使用；提到 mygo、vibrancy、毛玻璃、磨砂、亚克力、acrylic、透明窗口、背景不透明度时也用，即使没说「亚克力」这个词。
---

# mygo 亚克力 · Acrylic for mygo

一套从 Gloss（Git 客户端）里提炼出来、逐值验证过的 mygo 外观：窗口是磨砂的，
工作卡是半透明的，卡片上的一切都让材质透上来。组件库在 `assets/acrylic/`
（Go 包，自带测试与样板间），截图在 `assets/shots/`。

## 三层模型

| 层 | 画法 | 透不透 |
| :--- | :--- | :--- |
| 窗口底 Ground | 系统材质 + `Ground × 0.65` 薄纱；没有材质（Linux、旧 Windows、测试）实铺 `Ground` | 固定薄纱，不跟设置走 |
| 工作卡 Card | `Card × 用户不透明度`（30%~100%，默认 80%），圆角与窗口同心 | **唯一**的可调透明度 |
| 浮层 / 模态 / 代码画布 | 实色 `Popover` / `Modal` / `Code` | 本来就是实的 |

侧栏不铺底，直接坐在窗口底上；卡片里的面板、列表、工具栏不铺底，直接坐在卡片上。

## 三条铁律

1. **层拥有不透明度，控件只用洗色。** 不透明的颜色只许当上表五种层的底。卡片 /
   侧栏上的控件、条带只能用洗色阶梯（`WashHover` / `WashChip` / `WashSelected` /
   `WashKbd`）或干脆不铺底。理由：卡片一透，任何一支实色（哪怕和卡片同色）都会
   在磨砂上抠出一块挡板 —— 100% 时看不出来，往下一拧才露馅，截图评审抓不到。
2. **透明度只有一个入口。** 用户设置里的不透明度落在工作卡这一层；窗口底只有
   固定的 65% 主题色薄纱。组件里不手写 `.Alpha(…)` 去「让毛玻璃透上来」——各乘
   一次，最终颜色就交给了桌面壁纸。（语义淡染如 `Warning.Alpha(0.14)`、选中带
   `Selection` 是例外：它们本来就是半透明的语义色。）
3. **不拿底色去盖东西。** 底不再是一支已知的实色，「先铺一圈底色把线头盖掉」
   「用背景色画遮罩 / 光晕 / 渐隐」全都会在磨砂上留下实心环。留白、断开、遮挡
   一律用几何做（让线自己停住、端点退半个线宽给圆头）。

## 决策表：洗色阶梯

状态只在一条梯子上往上走，从轻到重：`透明 → WashHover → WashChip → WashSelected`。
不换几何（边框一直占 1px、只换颜色；不改字重 / 内边距）。

| 角色 | 静息 | 悬停 | 按下 / 选中 / 进行中 | 组件 |
| :--- | :--- | :--- | :--- | :--- |
| 面板、栏、列表、工具栏 | 不铺底 | — | — | 卡片里的一切容器 |
| 条带（抬一档） | `WashChip` | — | — | `HeaderStrip` `FooterStrip` |
| 幽灵控件 / 行 | 透明 | `WashHover` | `WashSelected` | `IconButton` `ToolButton` `Row` 分段格 |
| 实心控件 | `WashChip` + Hairline | `WashSelected` + HairlineStrong | 同悬停 | `Button` `SearchField` `Checkbox` 分段轨道 |
| 芯片 / 徽章 | `WashChip` + Hairline | — | — | `Chip`（语义只靠 6px 圆点） |
| 快捷键 | `WashKbd` | — | — | `Kbd` |
| 主按钮 | `Accent` 实心 | `AccentHover` | `AccentPressed` | `PrimaryButton`（一屏至多一颗） |
| 列表选中带 | `Selection`（强调淡染） | — | — | `Row(selected)` |

洗色**不挑底**：落在亚克力卡、实色弹窗、代码画布上都是「提一档」，所以共享控件
一律只用洗色，不用分场合。`palette.Wash(kind, hovered, active)` 就是这张表。

## 怎么用

**判断场景**，只读需要的参考：

- **新建 mygo 应用 / 新窗口外壳** → 第 1 步。
- **给已有亚克力应用写新控件** → 直接用组件；没有现成的，照决策表写，写完跑第 3 步的两项检查。
- **把现有界面改成亚克力兼容 / 排查抠穿** → 读 [references/refactor-playbook.md](references/refactor-playbook.md)，按它的盘点 → 分类 → 映射 → 守则 → 压力截图走。
- 要查具体色值 → [references/tokens.md](references/tokens.md)；组件 API 与配方 → [references/components.md](references/components.md)。

### 第 1 步 · 装上组件库

```bash
cp -R <本技能目录>/assets/acrylic <项目>/internal/acrylic   # 改 package 名不需要
cd <项目> && go test ./internal/acrylic/                    # 要求 mygo ≥ v0.4.8
```

### 第 2 步 · 接外壳

```go
opts := mygo.WindowOptions{Title: "App", Width: 1280, Height: 860,
	TitleBarStyle: mygo.TitleBarHiddenInset, Content: ui.View(app.view)}
acrylic.Window(&opts, isDark, acrylic.NormalizeBlur(cfg.Blur)) // Transparent + 首帧底色 + 材质
acrylic.SetAppearance(cfg.ThemeMode)                          // 原生材质跟应用的明暗走
win := mygo.NewWindow(opts)

func (app *App) view(c *ui.Context) {
	p := acrylic.For(app.dark(c))
	c.SetTheme(acrylic.Theme(p))  // 必须先于任何控件
	acrylic.Ground(c, p)          // 窗口底
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		app.sidebar(c, p)  // 不铺底
		ui.Column(c).Grow(1).FillHeight().Padding(acrylic.CardGap).Children(func() {
			acrylic.Card(c, p, app.opacity).Children(func() { /* 工具栏、面板：不铺底 */ })
		})
	})
}
```

运行中改设置：模糊档位 `win.SetVibrancy(blur.Vibrancy()); win.Invalidate()`；
不透明度只需 `win.Invalidate()`（卡片颜色每帧现算）；明暗 `acrylic.SetAppearance(mode)`。
设置页给两项：「背景透明度」滑杆（30%~100%）与「亚克力模糊」四档分段（无 / 柔和 / 标准 / 强力）。
`assets/acrylic/gallery.go` 是完整外壳的样板间，照它搭。

### 第 3 步 · 交付前检查（强制）

1. **压力截图**：窗口底换成品红、卡片拧到 30%，凡是没字的地方透不出品红就是抠穿：
   ```go
   tt := ui.NewTester(func(c *ui.Context) { app.view(c); c.Root().Background(ui.Hex("#ff00ff")) }, 1280, 860)
   ```
   每个视图（含选中、悬停、空状态、各种模式栏）深浅各拍一张，用眼看；只允许代码画布、
   浮层、模态是实的。组件库的 `TestComponentsLetTheMaterialThrough` 是这一步的自动版。
2. **静态守则**：把 `assets/acrylic/acrylic_guard_test.go` 复制进应用包、改三处设置（见
   [references/refactor-playbook.md](references/refactor-playbook.md) 第 5 节）—— 卡片 / 侧栏上的函数碰到不透明的层色或
   `ui.Checkbox` / `ui.Radio` / `ui.Split` 就报错，浮层与画布进白名单并写明理由。
   交付前做一次反向测试（故意塞一处 `p.Card` 进卡片控件，确认它报错）。
3. 正常外观（默认 80%、真实窗口底）深浅各看一眼：层次还在，不是一片糊。

## 反面（每条都在真实项目里踩过）

- ❌ `Background(p.Card)` / `Surface1` 铺在卡片里的面板上——「和卡片同色所以看不出来」只在 100% 时成立。
- ❌ 框架自带皮的控件放在卡片上：`ui.Checkbox` / `ui.Radio` 未勾选时铺 `t.Background`（实心方块）；`ui.Split` 拿 `t.Border` 画线，而亚克力主题把它设成透明——能拖的缝一根线都没有。
- ❌ 提交拓扑图节点外「先铺一圈行底色盖住线头」——浅色下成了一圈圈白斑。改为端点停在 `节点半径 + 留白 + 线宽/2`。
- ❌ 编辑器 / DiffView 画布跟着透——它的增删淡染从背景色现算，背景带 alpha 淡染也跟着变透，代码没有稳定对比度。代码画布保持 `Code` 实色。
- ❌ 只调视图主题不调原生外观——应用固定深色、系统是浅色时，材质还是浅的，侧栏亮成一片。
- ❌ 拖拽中的副本用洗色——框架按 75% 不透明度把它浮在指针上，洗色只剩一层看不见的雾；拖起来的那一枚按浮层取实色。
