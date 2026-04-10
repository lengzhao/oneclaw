package config

import "embed"

// init_template：全新项目 oneclaw -init 时整棵复制到 <cwd>/.oneclaw/（已存在的目标文件不覆盖）。
//
//go:embed all:init_template
var initTemplateFS embed.FS
