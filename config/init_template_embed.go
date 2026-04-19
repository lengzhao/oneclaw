package config

import "embed"

// init_template：`oneclaw -init` 时整棵复制到初始化目标的数据根（通常是 `~/.oneclaw/`；已存在的目标文件不覆盖）。
//
//go:embed all:init_template
var initTemplateFS embed.FS
