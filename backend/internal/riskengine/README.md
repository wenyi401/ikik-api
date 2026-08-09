# Risk Engine V2

Risk Engine V2 是独立的共享账号滥用识别实验，不读取或更新旧的统一风险分，也没有用户、API Key 或分组处罚执行器。

## 判定路径

1. 网关协议适配器只提供最后一轮、可确认由用户编写的文本。
2. 候选检测器只决定是否需要二阶段判断；初始影子实验使用 `AllTrafficDetector`，避免未经验证的关键词过滤造成漏判。
3. 领域裁决器输出 `safe`、`review`、`confirmed` 或 `abstain`，并同时给出分类、意图、可执行程度、授权状态和原文证据。
4. 策略层只生成 `WouldProtect`、`WouldStrike` 建议。影子存储的数据库约束固定为 `mode='shadow'`。
5. 相同用户、分组、分类和规范化文本使用稳定事件指纹聚合，重复请求不会产生新的独立事件。

## 数据命令

```bash
go run ./cmd/risk-engine-dataset prepare -input raw.jsonl -output review.jsonl
go run ./cmd/risk-engine-dataset label -review review.jsonl -labels labels.jsonl -output gold.jsonl
go run ./cmd/risk-engine-dataset readiness -dataset gold.jsonl
go run ./cmd/risk-engine-predict -endpoint http://127.0.0.1:11434/v1 -model qwen3:1.7b -dataset gold.jsonl -output predictions.jsonl
go run ./cmd/risk-engine-dataset evaluate -dataset gold.jsonl -predictions predictions.jsonl
```

生产原始文本只允许通过 `prepare` 管线进入本地忽略目录。管线会排除无法确定当前用户请求的 AGENTS、环境块、压缩历史和系统摘要，并脱敏邮箱、IP、令牌、密码及用户主目录。

## 模型路线

- 第一阶段先用 `qwen3:1.7b` 一类通用指令模型验证结构化裁决能力，不直接处罚。
- 数据达到训练门槛后，在有 CUDA GPU 的离线环境对 `Qwen3-0.6B` 或同级模型做 LoRA 蒸馏，再量化后部署到生产服务器推理。
- 生产服务器适合运行量化模型，不适合在 6 CPU/6 GB 限额内进行 LoRA 训练。
- 训练门槛默认是 500 条总样本、200 条真实生产样本、100 条确认违规、100 条测试样本，以及每个自动处理分类至少 20 条确认违规。
- 所有确认违规训练样本必须标注可以在原文中定位的证据；不满足门槛时不得训练或启用自动保护。

自动保护要求确认违规精确率至少 98%、召回率至少 90%；自动累计处罚要求精确率至少 99.5%。
