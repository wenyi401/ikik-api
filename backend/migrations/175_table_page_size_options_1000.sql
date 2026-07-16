UPDATE settings
SET value = '[10,20,50,100,1000]',
    updated_at = NOW()
WHERE key = 'table_page_size_options'
  AND regexp_replace(value, '\s+', '', 'g') = '[10,20,50,100]';
