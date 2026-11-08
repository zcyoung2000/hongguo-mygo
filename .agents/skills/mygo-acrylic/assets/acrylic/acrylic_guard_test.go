package acrylic

// 复制到应用里用：改 package 名，再改下面三处（paletteVar、opaqueFields、
// acrylicOwners）。白名单里这几条是组件库自己的，搬过去后删掉 —— 测试会提醒。

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// 亚克力守则：工作卡和侧栏上的函数不许直接用不透明的层色，也不许用自带
// 不透明底的框架控件。浮层、弹窗、设置窗口、代码画布进 acrylicOwners 白名单，
// 并写明理由 —— 加进来要付解释成本，这本身就是一道闸。
//
// 接进项目只改下面三处。

// paletteVar 是调色板变量在这个包里的名字（Gloss 叫 pal，组件库示例叫 p）。
// 统一一个名字，静态扫描才认得出来。
const paletteVar = "p"

// opaqueFields 是调色板里的不透明层色。用旧调色板（surface1~4、codeBg……）
// 的项目把那些字段名也列进来。
var opaqueFields = map[string]bool{
	"Ground": true, "Card": true, "Popover": true, "Modal": true, "Code": true,
}

// skinnedWidgets 是框架里自带不透明底、或在亚克力主题下画不出来的控件。
var skinnedWidgets = map[string]string{
	"ui.Checkbox":      "用 acrylic.Checkbox：框架那颗未勾选时铺实心 t.Background",
	"ui.Radio":         "照 acrylic.Checkbox 写一颗洗色单选：框架那颗未选中时铺实心 t.Background",
	"ui.Split":         "用 acrylic.Splitter：ui.Split 拿 t.Border 画线，亚克力主题里它是透明的",
	"ui.SplitVertical": "用 acrylic.Splitter（vertical=false）",
}

// acrylicOwners：键是「文件」或「文件:函数」，值是它为什么画在实色的层上。
var acrylicOwners = map[string]string{
	"tokens.go:Theme":           "框架主题的 Background：框架自绘控件读它，卡片上的勾选框已换成洗色版",
	"acrylic.go:GroundColor":    "窗口底这一层的定义",
	"acrylic.go:CardFill":       "工作卡这一层的定义",
	"acrylic.go:PopoverSurface": "下拉 / 右键浮层的面",
	"acrylic.go:Modal":          "模态面板",
	"gallery.go:Gallery":        "样板间里那块代码画布，按设计不透明",
}

func TestAcrylicLayersUseWashes(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, e := range entries {
		path := e.Name()
		if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			owner := ""
			for _, key := range []string{path + ":" + fn.Name.Name, path} {
				if _, ok := acrylicOwners[key]; ok {
					owner = key
					break
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var what, fix string
				switch n := n.(type) {
				case *ast.SelectorExpr:
					x, ok := n.X.(*ast.Ident)
					if !ok || x.Name != paletteVar || !opaqueFields[n.Sel.Name] {
						return true
					}
					what = paletteVar + "." + n.Sel.Name
					fix = "面板 / 条带不铺底；控件静息 WashChip（幽灵控件透明）、悬停 WashHover、选中与按下 WashSelected"
				case *ast.CallExpr:
					var b bytes.Buffer
					_ = printer.Fprint(&b, fset, n.Fun)
					hint, ok := skinnedWidgets[b.String()]
					if !ok {
						return true
					}
					what, fix = b.String(), hint
				default:
					return true
				}
				if owner != "" {
					used[owner] = true
					return true
				}
				t.Errorf("%s: %s 用了 %s —— 在亚克力层上会抠出一块实心；%s。"+
					"确实画在浮层、弹窗或代码画布上的话，加进 acrylicOwners 并写明理由",
					fset.Position(n.Pos()), fn.Name.Name, what, fix)
				return true
			})
		}
	}
	for key := range acrylicOwners {
		if !used[key] {
			t.Errorf("acrylicOwners[%q] 里已经没有不透明的面了 —— 删掉这一条", key)
		}
	}
}
