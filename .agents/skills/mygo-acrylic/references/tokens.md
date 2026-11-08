# Tokens · 色值与角色

唯一事实来源是 `assets/acrylic/tokens.go`（`acrylic.Dark()` / `acrylic.Light()`）。
这里是给人和 agent 查的表；改值改 Go 文件，再同步这张表。取值与 Gloss v0.3.25 之后逐值一致。

## 层（不透明，只许当一层的底）

| Token | 深色 | 浅色 | 角色 |
| :--- | :--- | :--- | :--- |
| `Ground` | `#141418` | `#eceef4` | 窗口底。有材质时乘 `GroundVeil = 0.65` 当薄纱；没有材质实铺 |
| `Card` | `#232328` | `#ffffff` | 工作卡实色，乘用户不透明度（`CardFill`） |
| `Popover` | `#232327` | `#ffffff` | 下拉、右键浮层 |
| `Modal` | `#2b2b31` | `#ffffff` | 模态面板 |
| `Code` | `#232328` | `#ffffff` | 代码 / diff 画布（淡染从它现算，必须实） |

浅色的窗口是「灰的」、卡片是「白的」：层次靠两者的明度差和卡片外沿那条 1px 硬边。

## 洗色阶梯（半透明，控件与条带的底）

深色叠白、浅色叠黑；从轻到重排：

| Token | 深色 | 浅色 | 用在 |
| :--- | :--- | :--- | :--- |
| `WashHover` | `rgba(255,255,255,0.05)` | `rgba(0,0,0,0.04)` | 幽灵控件 / 行的悬停 |
| `WashKbd` | `rgba(255,255,255,0.06)` | `rgba(0,0,0,0.04)` | 快捷键提示 |
| `WashChip` | `rgba(255,255,255,0.07)` | `rgba(0,0,0,0.045)` | 实心控件静息、芯片、输入框、条带、分段轨道 |
| `WashSelected` | `rgba(255,255,255,0.10)` | `rgba(0,0,0,0.07)` | 选中 / 按下 / 实心控件悬停 |

压在 100% 的卡片上，它们约等于原来的不透明阶梯：`WashHover ≈ Surface 2`（深色 +11 阶 /
浅色 −10 阶），`WashSelected` 比它再重一档。所以从不透明阶梯迁过来，观感基本不变。

## 发丝线（1px，三档）

| Token | 深色 | 浅色 | 用在 |
| :--- | :--- | :--- | :--- |
| `Hairline` | `rgba(255,255,255,0.07)` | `rgba(0,0,0,0.06)` | 分割线、芯片边、静息控件边、分隔条 |
| `HairlineStrong` | `rgba(255,255,255,0.11)` | `rgba(0,0,0,0.10)` | 输入框外框、悬停 / 选中控件边 |
| `HairlineTertiary` | `rgba(255,255,255,0.16)` | `rgba(0,0,0,0.14)` | 输入框悬停、弹窗外围 |

## 墨（四档灰阶）

| Token | 深色 | 浅色 | 用在 |
| :--- | :--- | :--- | :--- |
| `Ink` | `#ececf0` | `#1d1d1f` | 标题、正文基准、选中 / 悬停项 |
| `InkMuted` | `#a9a9b4` | `#6e6e78` | 常规正文、按钮文字 |
| `InkSubtle` | `#83838f` | `#8d8d97` | 元信息、未激活图标 |
| `InkTertiary` | `#656570` | `#a3a3ad` | 占位符、禁用、快捷键 |

深色窗口底压在最亮的材质（实测约 `#898c8d`）上，`InkMuted` 仍有 ≥ 4.5:1 —— 这是 0.65
薄纱的来由（`TestGroundVeil`）。改 `Ground` 或 `GroundVeil` 先跑这条。

## 强调与语义

| Token | 深色 | 浅色 | 规矩 |
| :--- | :--- | :--- | :--- |
| `Accent` / `Hover` / `Pressed` | `#6b93ff` / `#82a3ff` / `#5b82ee` | `#3b6cf6` / `#527ef7` / `#2f5ddb` | 一支蓝；全屏同时亮的实心强调 ≤ 1~2 处 |
| `AccentText` | `#ffffff` | `#ffffff` | 强调色上的字 |
| `Selection` | `rgba(107,147,255,0.14)` | `rgba(59,108,246,0.12)` | 列表选中带 |
| `Focus` | `rgba(107,147,255,0.65)` | `rgba(59,108,246,0.55)` | 键盘焦点圈 |
| `Success` / `Warning` / `Danger` | `#27a644` / `#f0a33a` / `#ff6961` | `#22863a` / `#c27b0f` / `#d93a35` | 只用于语义；横幅底压到 α≈0.10 |

## 阴影与硬边

| Token | 深色 | 浅色 |
| :--- | :--- | :--- |
| `CardBorder` | `rgba(255,255,255,0.08)` | `rgba(0,0,0,0.07)` |
| `CardShadow` / `CardShadow2` | `rgba(0,0,0,0.35)` / `0.20` | `rgba(0,0,0,0.07)` / `0.035` |
| `FloatShadow` | `rgba(0,0,0,0.45)` | `rgba(20,20,40,0.14)` |

浅色卡片与浮层外沿那条 1px 硬边用 `Shadow(0, 0, 0, 1, CardBorder)` 画，不用 `Border`：
`Border` 会像内边距一样把内容往里挤 1px。深色用「上沿亮边 + 三层收紧的影子」（`Raised`）。

## 形状

| Token | 值 | 绑定组件 |
| :--- | :--- | :--- |
| `RadiusXS` | 4 | 勾选框、极小标签、快捷键 |
| `RadiusSM` | 6 | 幽灵按钮、芯片、分段格、列表选中带 |
| `RadiusMD` | 8 | 通用按钮、输入框、搜索栏、分段轨道 |
| `RadiusLG` | 12 | 卡片、浮层、Composer |
| `RadiusXL` | 16 | 模态 |
| 卡片圆角 | `max(窗口圆角 − 8, 12)` | macOS 26 窗口 28 → 20；Windows 8 → 12；Linux 12 → 12 |

## 设置

| 项 | 值 | 存配置 |
| :--- | :--- | :--- |
| 背景透明度 | 0.30 ~ 1.0，默认 0.80；0 / 越界 → `NormalizeOpacity` | `float64`，`omitempty` |
| 亚克力模糊 | `none` / `subtle`（默认）/ `medium` / `deep` → `None` / `Sidebar` / `Popover` / `Acrylic` 材质 | `string`，坏值回落 `subtle` |

## 和 Gloss 的对照

在 Gloss 仓库里改东西时，名字对得上：`pal.chip` = `WashChip`、`pal.rowHover` = `WashHover`、
`pal.rowSelected` = `WashSelected`、`pal.kbd` = `WashKbd`、`pal.sideBg` = `Ground`、
`pal.card` = `Card`、`pal.popover` = `Popover`、`pal.surface4` = `Modal`、`pal.codeBg` = `Code`、
`shellGround` = `GroundColor`、`cardFill` = `CardFill`、`shellTint` = `GroundVeil`、
`washCheckbox` = `Checkbox`。Gloss 的规范全文在它仓库的 `DESIGN.md` §2.1「洗色阶梯」与 §5。
