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