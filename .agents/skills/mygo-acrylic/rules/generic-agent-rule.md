# mygo Acrylic（通用 Agent 指令）

> **适用平台**：Windsurf (Cascade)、Cline / Roo Code、GitHub Copilot、Zed，或任何支持自定义 System Instructions 的 AI 编程助手。

## 角色与目标

你在 mygo（github.com/egoist/mygo）原生 UI 上实现或维护「亚克力」外观：窗口是系统磨砂的，工作卡是半透明的，卡片上的一切都让材质透上来。你对「在磨砂上铺一块不透明底」零容忍——它在卡片 100% 不透明时看不出来，用户一拧透明度才露馅。

## 核心规则

1. **三层**：窗口底 = 系统材质 + 主题色 65% 薄纱（无材质平台实铺）；工作卡 = 卡片色 × 用户不透明度（30%~100%，默认 80%）；浮层 / 模态 / 代码画布 = 实色。
2. **层拥有不透明度，控件只用洗色**：卡片和侧栏上的容器不铺底；控件与条带只用洗色阶梯——幽灵控件 透明 → 悬停 WashHover → 按下 / 选中 WashSelected；实心控件 WashChip → 悬停 / 按下 WashSelected；条带、芯片 WashChip；快捷键 WashKbd。洗色不挑底，共享控件一律只用它。
3. **透明度只有一个入口**：用户设置里的不透明度落在卡片；组件里不手写 `.Alpha()` 让毛玻璃透上来（语义淡染除外）。
4. **不拿底色盖东西**：光晕、底色圈、遮罩一律改成几何留白。
5. **框架控件**：卡片上不用 `ui.Checkbox` / `ui.Radio` / `ui.Split`，换成洗色版本；编辑器 / DiffView 画布保持实色；终端插件开 `Transparent`。
6. **原生侧**：`WindowOptions{Transparent: true, BackgroundColor: 窗口底色, Vibrancy: 档位}`；`mygo.Theme.SetSource` 跟应用明暗；改档位 `win.SetVibrancy`。
7. **交付前强制检查**：品红窗口底 + 30% 卡片的深浅截图（没字处必须透出品红，只许代码画布 / 浮层是实的）；静态守则测试 + 反向测试；正常外观再看一眼层次。

## 组件库

完整的 Go 组件库（tokens、外壳、按钮、分段、输入、勾选框、芯片、列表行、条带、分隔条、模态、样板间与测试）在技能目录 `assets/acrylic/`，复制进项目即可使用；色值表见 `references/tokens.md`，改造现有项目的步骤见 `references/refactor-playbook.md`。
