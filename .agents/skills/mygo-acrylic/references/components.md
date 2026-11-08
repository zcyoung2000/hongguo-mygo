# Components · 组件 API 与配方

全部在 `assets/acrylic/`（`package acrylic`）。签名里的 `p` 是 `acrylic.Palette`，
每帧 `acrylic.For(dark)` 取一份（值类型，不分配）。按钮都返回 `ui.Element`，调用方问
`.Clicked()`；Label 就是无障碍名字，也是测试 `tt.Find` / `tt.Click` 的凭据。

## 外壳

| API | 做什么 |
| :--- | :--- |
| `Window(&opts, dark, blur)` | `Transparent = true`、`BackgroundColor = Ground`（首帧前露出的色）、`Vibrancy = blur.Vibrancy()` |
| `SetAppearance(mode)` | `"light"` / `"dark"` / 其他 → `mygo.Theme.SetSource`，原生材质跟应用明暗走 |
| `Theme(p)` | 配套的 `*ui.Theme`（`Border` 透明、Accent / Focus / 语义色对齐）；先于任何控件 `c.SetTheme` |
| `Ground(c, p)` | 根铺窗口底：`c.Vibrancy()` 为真铺薄纱，否则实铺 |
| `Card(c, p, opacity)` | 工作卡：`CardFill` + 同心圆角 + `Clip` + `Raised`。里面的容器都不铺底 |
| `Raised(e, p)` / `Floating(e, p)` | 卡片 / 浮层的抬升（硬边 + 影子） |
| `PopoverSurface(e, p)` | 下拉 / 右键浮层的实色面 |
| `Modal(c, p, &open, fn)` | `ui.DialogBase` 上铺 `Modal` 面板（圆角 XL、压暗背景） |
| `Fade(e)` | 120ms 颜色过渡，给「平时没脸、悬停才上色」的元素 |
| `Icon(c, svg, size, color)` / `Lucide(paths)` | 单色描边图标 |

## 控件

| API | 阶梯 | 说明 |
| :--- | :--- | :--- |
| `IconButton(c, p, label, icon, disabled)` | Ghost | 28×28，图标 15，tooltip = label |
| `ToolButton(c, p, label, icon, disabled)` | Ghost | 图标 + 文字，高 28 |
| `Button(c, p, label, disabled)` | Filled | 次级键：WashChip + 发丝边 + InkMuted |
| `PrimaryButton(c, p, label, disabled)` | 实心强调 | 一屏 / 一个对话框至多一颗；禁用退成 WashChip |
| `SegmentedText(c, p, &sel, labels...)` | 轨道 Filled，格 Ghost | 选中格描 HairlineStrong；格的边一直占 1px |
| `SegmentedIcons(c, p, &sel, Segment{…}...)` | 格 Ghost | 轨道不铺底不描边（工具栏上不要「框里的框」） |
| `SearchField(c, p, &value, placeholder)` | Filled | 返回内层输入元素；边：静息 Strong → 悬停 Tertiary → 聚焦 Accent |
| `Checkbox(c, p, &checked, label)` | Filled | 代替 `ui.Checkbox`（后者未勾选铺实心 `t.Background`） |
| `Chip(c, p, label, dot)` | 芯片 | 语义只靠 6px 圆点；`dot = ui.Transparent` 不画 |
| `Kbd(c, p, "⌘K")` | WashKbd | 快捷键提示 |
| `Row(c, p, selected)` | Ghost 行 | 悬停 WashHover，选中 `Selection` 淡染带；接 `.Children` |
| `HeaderStrip(c, p)` / `FooterStrip(c, p)` | 条带 | WashChip + 下 / 上沿发丝线 |
| `Splitter(c, p, &size, lo, hi, reset, vertical)` | 线 | 1px 发丝线 + 6px 命中区；双击复位；代替 `ui.Split` |
| `p.Wash(kind, hovered, active)` / `p.Edge(hovered, active)` | — | 自己写控件时取这一帧的底与边 |

## 自己写一个控件的配方

```go
func Tag(c *ui.Context, p acrylic.Palette, label string, on bool) ui.Element {
	t := ui.ButtonBase(c).Height(24).Padding(0, 8).Radius(acrylic.RadiusSM).
		Cursor(ui.CursorPointer).Label(label).Tooltip(label) // Label/Tooltip 先于 Disabled
	hov := t.Hovered()
	fg := p.InkMuted
	if on || hov {
		fg = p.Ink
	}
	t.Background(p.Wash(acrylic.Ghost, hov, on)).
		Border(1, map[bool]ui.Color{true: p.HairlineStrong, false: ui.Transparent}[on]) // 边一直占 1px
	acrylic.Fade(t)
	t.Children(func() { ui.Text(c, label).FontSize(acrylic.TextButton).TextColor(fg) })
	return t
}
```

检查清单：底只来自 `p.Wash*` / 不铺；状态不改几何；禁用不描边（一枚灰按钮还描着边，看着仍像能点）；
有 tooltip 的禁用控件把 `Tooltip` 写在 `Disabled` 前面。

## 哪些地方保持实色（不是违规）

| 地方 | 为什么 |
| :--- | :--- |
| 代码 / diff 画布（`editor.DiffView`、`editor` 主题的 `Background = p.Code`） | 增删淡染按背景色现算；背景带 alpha，淡染也跟着透，文字没有稳定对比度 |
| 浮层、右键菜单、模态面板、设置窗口 | 浮在内容上，要让底下的代码保持可读；里面的控件仍可用洗色（洗色不挑底） |
| 拖拽中浮在指针上的副本 | 框架按 75% 不透明度画它；洗色在那儿只剩一层看不见的雾 |
| 图片预览的棋盘格 | 它就是内容画布，看透明像素用的 |

终端（`plugins/terminal`）不在此列：`terminal.Options{Transparent: true}` 让画布不铺底，
主题的 `Background` 仍要给（块状光标下的字靠它取反色），外层容器也不铺底。

## 设置页的两项

```go
// 背景透明度：直接拖 0.30~1.0 的小数，按 5% 一格；百分比文字自己画在旁边。
if ui.Slider(c.Key("bg-opacity"), &cfg.Opacity, acrylic.OpacityMin, acrylic.OpacityMax).
	Step(0.05).Width(150).Changed() {
	cfg.Opacity = acrylic.NormalizeOpacity(cfg.Opacity)
	save(cfg)
	win.Invalidate() // 卡片颜色每帧现算，重画就够
}
// 亚克力模糊：四档分段，存 id 不存下标。
idx := slices.Index(acrylic.Blurs, acrylic.NormalizeBlur(cfg.Blur))
if acrylic.SegmentedText(c, p, &idx, "无", "柔和", "标准", "强力").Changed() {
	cfg.Blur = string(acrylic.Blurs[idx])
	save(cfg)
	win.SetVibrancy(acrylic.Blurs[idx].Vibrancy()) // 材质是原生那一侧的，必须重设
	win.Invalidate()
}
```

文案提醒一句平台差异：「窗口用的是系统磨砂材质（macOS / Windows 11；其他平台退回纯色底），
按当前主题染色。」—— Linux 用户拧了滑杆看不见磨砂，别让他以为坏了。
