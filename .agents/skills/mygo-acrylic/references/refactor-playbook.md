# Refactor playbook · 把现有界面改成亚克力兼容

Gloss 走过一遍（约 60 处、19 个文件）。顺序很重要：**先改共享控件**（一改就连带
修好几十个调用点），再改面板，最后处理特殊情况；每一步编译 + 跑测试。

## 0 · 先接好三层

缺哪样补哪样（组件库 `acrylic.Window` / `SetAppearance` / `Ground` / `Card` 一次给齐）：

- `WindowOptions.Transparent = true` + `BackgroundColor`（首帧前的底色，取窗口底那支）+ `Vibrancy`。
- `mygo.Theme.SetSource(...)` 跟应用的明暗：应用固定深色、系统浅色时，材质还是浅的。
- 根铺 `GroundColor(p, c.Vibrancy())`；卡片铺 `CardFill(p, opacity)`；侧栏、卡片里的容器不铺底。
- 运行中改模糊档位必须 `win.SetVibrancy(...)`（材质在原生那一侧），改不透明度只需重画。

## 1 · 盘点

```bash
# 不透明的层色 / 阶梯（把字段名换成项目里的）
grep -nE '\b(pal|p)\.(surface[1-4]|card|Card|popover|Popover|codeBg|Code|appBg|control|sideBg|Ground)\b' *.go | grep -v _test.go
# 框架自带皮的控件
grep -nE 'ui\.(Checkbox|Radio|Split|SplitVertical|Button|TextInput|SearchField)\(' *.go | grep -v _test.go
# 「拿底色盖东西」：光晕、遮罩、底色圈
grep -nE 'halo|knockout|nodeBg|FillPath\(&[a-z]+, *bg\)|Background\(bg\)' *.go | grep -v _test.go
# 组件里手写的 α（逐条判断是不是语义淡染）
grep -nE '\.Alpha\(0\.[0-9]+\)' *.go | grep -v _test.go
```

给每处标上所在函数，一眼分层：

```bash
grep -nE '<上面的模式>' *.go | grep -v _test.go | while IFS=: read f n _; do
  printf '%-22s %-5s %s\n' "$f" "$n" "$(awk -v n=$n 'NR<=n && /^func /{x=$0} END{print x}' "$f" | sed -E 's/^func (\([^)]*\) )?([A-Za-z0-9_]+).*/\2/')"
done
```

## 2 · 分类

每处问一句「它画在哪一层上」：

| 落在 | 处理 |
| :--- | :--- |
| 工作卡、侧栏（工具栏、文件栏、列表、栏头、页脚、Composer、空状态页、模式栏） | **改** |
| 共享控件（按钮、分段、× 键、芯片、勾选框）—— 既上卡片也进弹窗 | **改**（洗色不挑底，两边都成立） |
| 下拉 / 右键浮层、模态、设置窗口**自己的**嵌块与条目 | 留着（底是实的），进守则白名单 |
| 代码 / diff 画布、图片棋盘格、拖拽副本 | 留着，进白名单 |

## 3 · 映射

| 原来 | 它在干嘛 | 换成 |
| :--- | :--- | :--- |
| Surface 1（= 卡片色）当面板 / 条 / 容器底 | 和卡片「平齐」 | **不铺底**（描边留着） |
| Surface 1/2 当控件静息底（按钮、输入框、轨道、芯片、头像） | 「凸起一档」 | `WashChip` |
| Surface 2 当栏头 / 页脚 / 状态条 | 「抬一档」的条带 | `WashChip` + 发丝线 |
| Surface 2 当幽灵控件 / 行的悬停 | 悬停 | `WashHover` |
| Surface 2/3 当实心控件的悬停 / 按下 / 进行中 | 激活 | `WashSelected` + HairlineStrong |
| Surface 3 当分段格 / 标签的选中 | 选中 | `WashSelected` + HairlineStrong |
| 「槽」色（比卡片沉一档的带子） | 下沉 | **不铺底** + 发丝线（深色没有「下沉」的洗色，别硬造） |
| 快捷键底 | — | `WashKbd` |
| 禁用的主按钮底 | — | `WashChip` |

迁完后观感基本不变：`WashHover` 压在实色卡上约等于 Surface 2，`WashSelected` 再重一档。

## 4 · 特殊情况（都踩过）

- **框架勾选框 / 单选**：`ui.Checkbox` 未勾选时方框铺 `t.Background`。用 `ui.CheckboxBase` 自己画框（组件库 `Checkbox`），尺寸照框架那颗取（`t.Space(4)` 方框、`t.Space(2)` 间距），焦点圈用 `FocusRing(false)` + `DrawOver` 里 `p.FocusRing`。
- **分隔条**：亚克力主题把 `t.Border` 设成透明，`ui.Split` 的线跟着没了。自己画 1px 发丝线 + 6px 命中区（`Splitter`），命中区最后建，压在两栏内容上。
- **「底色圈」/ 光晕**（拓扑图节点、徽标镂空）：不再拿底色盖。线自己停在 `半径 + 留白 + 线宽/2`（圆头的弧顶正好落在外沿）；挤到同一列、会穿过节点的贯穿线在节点上下断开。`StrokePath` 永远是圆头。
- **终端插件**：`terminal.Options{Transparent: true}`，外层容器不铺底；主题 `Background` 仍要给（块状光标取反色）。
- **编辑器 / DiffView 画布**：保持实色。它的增删淡染从背景色现算，背景带 alpha 淡染也变透。卡片的亚克力留给工具栏、空白区、侧栏。
- **拖拽副本**：框架按 75% 不透明度把被拖元素的副本浮在指针上——那一态保留实色（浮层），原位芯片 `Opacity(0)`，落点空位用 `WashChip`。
- **浅色卡片 / 浮层的边**：只靠软阴影在灰底上会化开，补 `Shadow(0, 0, 0, 1, CardBorder)` 硬边；别用 `Border`（会挤内容 1px）。
- **死字段**：迁完后没人用的不透明色（Gloss 的 `appBg`、从没赋过值的 `fileBg`）连同测试与文档一起删，免得下一个人又拿来铺底。
- **语义淡染**（`Warning.Alpha(0.14)` 的冲突框悬停、横幅 `Danger` 0.10）本来就半透明，不用改。

## 5 · 守则测试（防回退）

组件库自带的 `assets/acrylic/acrylic_guard_test.go` 就是这道闸（组件库用它守自己）。复制进
应用包，改 package 名和三处设置：调色板变量名 `paletteVar`、不透明字段表 `opaqueFields`、
白名单 `acrylicOwners`（浮层 / 弹窗 / 设置 / 画布，键 `文件` 或 `文件:函数`，值写理由；组件库
自己的那几条删掉）。它会：

- 卡片 / 侧栏函数里出现不透明层色或 `ui.Checkbox` / `ui.Radio` / `ui.Split` → 报错并给出换法；
- 白名单里某条已经用不到不透明色 → 要求删掉（名单只减不增）。

接上后做两次反向测试：塞一处 `p.Card` 进卡片控件、加一条假白名单，确认都报错，再还原。

## 6 · 压力截图（看漏网的）

```go
// 只在 SHOT=1 时跑；窗口底换品红、卡片拧到 30%
func TestDumpAcrylicPunchOuts(t *testing.T) {
	if os.Getenv("SHOT") == "" {
		t.Skip("设置 SHOT=1 才跑")
	}
	app := newTestApp(t)
	app.opacity = 0.30
	tt := ui.NewTester(func(c *ui.Context) {
		app.view(c)
		c.Root().Background(ui.Hex("#ff00ff")) // 在 view 之后设，盖掉窗口底
	}, 1280, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true}) // 动效直接到终点
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		writePNG(t, fmt.Sprintf("/tmp/acrylic-%v.png", dark), tt.Image())
	}
}
```

每个视图、每种模式（选中、悬停、空状态、对比 / 检视模式栏、终端、编辑器）深浅各一张。
品红里浮出的灰 / 白实心块，除了代码画布、浮层、模态，全是漏网的。再把 `opacity` 设回
默认、去掉品红拍一组，确认正常外观下层次还在。

## 7 · 收尾

- 全量测试；像素断言里取色点的坐标若写死在旧的不透明色上，按新外观改。
- 同步设计文档：洗色阶梯表、组件条文里的 Surface 名字、「透明度只有一个入口」、守则测试的存在。
- 说明里写清哪些地方**故意**保持实色（代码画布、浮层），免得下一个人「顺手」把它们也改透。
