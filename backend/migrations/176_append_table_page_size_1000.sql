UPDATE settings
SET value = (
        SELECT jsonb_agg(option_value ORDER BY sort_order)::text
        FROM (
            SELECT item.value::int AS option_value, item.ordinality AS sort_order
            FROM jsonb_array_elements_text(settings.value::jsonb) WITH ORDINALITY AS item(value, ordinality)
            UNION ALL
            SELECT 1000, 1000000
        ) merged_options
    ),
    updated_at = NOW()
WHERE key = 'table_page_size_options'
  AND jsonb_typeof(value::jsonb) = 'array'
  AND NOT (value::jsonb @> '[1000]'::jsonb);
