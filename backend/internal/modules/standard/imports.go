// Package standard 是插件模块的唯一插装清单（instrumentation list）。
//
// 所有需要编译进二进制的内置模块必须且只能在本文件中匿名 import：
// 模块包在 init() 期向包级默认注册表自注册，cmd/server 匿名 import
// 本包即完成全部内置模块的插装。新增/移除模块只需修改本文件，
// 使插装变更集中、可 review。
package standard

import (
	// job.hello：示例模块（默认 disabled），验证插件链路连通性。
	_ "ikik-api/internal/modules/hello"
)
