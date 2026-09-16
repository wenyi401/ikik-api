-- 239: 恢复 groups 认证快照失效触发器的细粒度条件（含利润控制列）。
--
-- 背景：193_group_profit_control_auth_cache_invalidation.sql 把
-- enqueue_group_auth_cache_invalidation() 扩展为「13 个快照字段任一变化即入队」，
-- 覆盖 allow_image_generation / platform / subscription_type / rate_multiplier /
-- peak_* / profit_control_*。但此后的 203_auth_cache_invalidation_outbox.sql 用
-- 同一函数名重新定义，条件收敛成只剩 status / is_exclusive / deleted_at，
-- 于是 193 的细粒度判定（含利润控制）在运行时被静默覆盖：
-- 直接改库（后台裸 SQL、崩溃于应用侧失效之前）时，改 allow_image_generation 或
-- profit_control_* 不再入队，缓存的认证快照会带着旧值继续放行，利润门/图片门形同失效。
--
-- 本迁移把 193 的条件列表并回 203 的耐用结构（保留 api_key_group_routes 感知的
-- enqueue_group_auth_cache_invalidations(...) 与 DELETE 分支），不做别的改动。
-- 迁移文件不可改（checksum 记账），因此以追加迁移的方式修复。

CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.platform IS NOT DISTINCT FROM NEW.platform
       AND OLD.subscription_type IS NOT DISTINCT FROM NEW.subscription_type
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.peak_rate_enabled IS NOT DISTINCT FROM NEW.peak_rate_enabled
       AND OLD.peak_start IS NOT DISTINCT FROM NEW.peak_start
       AND OLD.peak_end IS NOT DISTINCT FROM NEW.peak_end
       AND OLD.peak_rate_multiplier IS NOT DISTINCT FROM NEW.peak_rate_multiplier
       AND OLD.profit_control_enabled IS NOT DISTINCT FROM NEW.profit_control_enabled
       AND OLD.profit_min_margin IS NOT DISTINCT FROM NEW.profit_min_margin
       AND OLD.profit_safety_buffer IS NOT DISTINCT FROM NEW.profit_safety_buffer
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    PERFORM enqueue_group_auth_cache_invalidations(target_group_id);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
