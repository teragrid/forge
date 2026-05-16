-- Canonical fixture: triggers performance and RLS security scanners.

-- performance: select-star rule
SELECT * FROM users WHERE active = 1;

-- performance: fk-missing-index (REFERENCES without -- indexed comment)
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id)
);

-- security/RLS: select-without-where-tenant (no tenant/workspace column)
SELECT id, name FROM products;
