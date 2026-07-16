ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.models_list_config IS '自定义 /v1/models 展示列表配置；仅用于控制模型列表响应，不参与账号白名单、模型映射或网关调度。';
