CREATE TABLE IF NOT EXISTS promos (
    id VARCHAR(50) PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    discount_type VARCHAR(20) NOT NULL,
    discount_value DECIMAL(10, 2) NOT NULL,
    min_purchase_amount DECIMAL(10, 2),
    max_uses INT DEFAULT NULL,
    used_count INT DEFAULT 0,
    expired_at TIMESTAMP NOT NULL,
    user_specific BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_promos_updated_at BEFORE
UPDATE ON promos FOR EACH ROW EXECUTE FUNCTION update_updated_at_column ();

CREATE TABLE IF NOT EXISTS user_promo_usage (
    id VARCHAR(50) PRIMARY KEY,
    user_id VARCHAR(50) NOT NULL,
    promo_id VARCHAR(50) NOT NULL,
    order_id VARCHAR(50) NOT NULL,
    used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (promo_id) REFERENCES promos (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE CASCADE ON UPDATE CASCADE,
    UNIQUE (user_id, promo_id)
);

CREATE TABLE IF NOT EXISTS promo_user_restrictions (
    id VARCHAR(50) PRIMARY KEY,
    promo_id VARCHAR(50) NOT NULL,
    user_id VARCHAR(50) NOT NULL,
    FOREIGN KEY (promo_id) REFERENCES promos (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    UNIQUE (promo_id, user_id)
);
