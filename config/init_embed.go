package config

import _ "embed"

// project_init.example.yaml：仓库唯一 YAML 配置示例，供 oneclaw -init 与文档引用。
//go:embed project_init.example.yaml
var projectInitExampleYAML []byte
