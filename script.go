package main

import (
	"os"
	"regexp"
	"strings"
)

// ScriptLine is a single teleprompter line.
type ScriptLine struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
	Read bool   `json:"read"`
}

// defaultReadMD is written to read.md on first launch, guiding new users to
// replace it with their own script.
const defaultReadMD = `# 欢迎使用 TipReader 提词器

这是软件第一次启动时自动生成的 read.md 示例稿件。

请用记事本或任意文本编辑器打开 exe 所在目录下的 read.md，把这里的内容全部替换成你自己的台词。

每一行文字就是提词器上的一行。

空一行可以分段，产生适当间距。

# 快捷操作

点击任意一行：从该行开始阅读，该行及之前的行会标红。

方向键上下或空格键：切换上一行、下一行。

Ctrl + R：修改并保存 read.md 后重新读取，无需重启软件。

Esc：关闭窗口。

顶部透明条可拖动窗口，设置面板可调整字号、颜色、行距等。

现在就动手，把这份示例换成你的第一份提词稿件吧！
`

// ensureReadMD creates read.md with the starter guide when it does not exist.
// An existing read.md is never overwritten.
func ensureReadMD(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, []byte(defaultReadMD), 0644)
}

var (
	reHeading = regexp.MustCompile(`^#{1,6}\s*`)
	reBold    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic  = regexp.MustCompile(`\*(.+?)\*`)
	reCode    = regexp.MustCompile("`([^`]+?)`")
	reLink    = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	reBullet  = regexp.MustCompile(`^\s*[-*+]\s+`)
	reNumber  = regexp.MustCompile(`^\s*\d+[.、)]\s*`)
)

// cleanLine strips Markdown markers and leaves readable text.
func cleanLine(line string) string {
	line = reHeading.ReplaceAllString(line, "")
	line = reBold.ReplaceAllString(line, "$1")
	line = reItalic.ReplaceAllString(line, "$1")
	line = reCode.ReplaceAllString(line, "$1")
	line = reLink.ReplaceAllString(line, "$1")
	line = reBullet.ReplaceAllString(line, "")
	line = reNumber.ReplaceAllString(line, "")
	return strings.TrimSpace(line)
}

// loadScript reads the markdown at mdPath, cleans it and returns ScriptLines.
// Rules (spec §14-17):
//   - headings keep uniform font size (handled in CSS)
//   - bold/italic/code/link markers are removed
//   - consecutive blank lines are collapsed to a single blank line
// Blank spacer rows get ID -1 and are not valid cursor targets.
func loadScript(mdPath string) ([]ScriptLine, error) {
	b, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	raw := strings.Split(s, "\n")

	var out []ScriptLine
	id := 0
	prevBlank := false
	wasHeading := false
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// collapse consecutive blank lines; keep at most one spacer
			if wasHeading && !prevBlank {
				// heading needs a visual gap below it
				out = append(out, ScriptLine{ID: -1, Text: ""})
				prevBlank = true
			} else if !prevBlank {
				out = append(out, ScriptLine{ID: -1, Text: ""})
				prevBlank = true
			}
			wasHeading = false
			continue
		}
		clean := cleanLine(trimmed)
		if clean == "" {
			// after stripping markers the content may be empty; treat as spacer
			if !prevBlank {
				out = append(out, ScriptLine{ID: -1, Text: ""})
				prevBlank = true
			}
			wasHeading = false
			continue
		}
		out = append(out, ScriptLine{ID: id, Text: clean})
		id++
		prevBlank = false
		wasHeading = strings.HasPrefix(trimmed, "#")
	}
	if len(out) == 0 {
		out = []ScriptLine{{ID: 0, Text: "（稿件为空）"}}
	}
	return out, nil
}