-- Drop existing triggers (if they exist)
DROP TRIGGER IF EXISTS update_audit_events_updated_at ON audits;

DROP TRIGGER IF EXISTS update_wishlists_updated_at ON wishlists;

DROP TRIGGER IF EXISTS update_category_updated_at ON category;

DROP TRIGGER IF EXISTS update_users_updated_at ON users;

DROP TRIGGER IF EXISTS update_product_features_updated_at ON product_features;

DROP TRIGGER IF EXISTS update_vendor_users_updated_at ON vendor_users;

DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;

DROP TRIGGER IF EXISTS update_reviews_updated_at ON reviews;

DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;

DROP TRIGGER IF EXISTS update_carts_updated_at ON carts;

DROP TRIGGER IF EXISTS update_admin_users_updated_at ON admin_users;

DROP TRIGGER IF EXISTS update_product_images_updated_at ON product_images;

DROP TRIGGER IF EXISTS update_order_items_updated_at ON order_items;

DROP TRIGGER IF EXISTS update_normal_users_updated_at ON normal_users;

DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;

DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;

DROP TRIGGER IF EXISTS update_cart_items_updated_at ON cart_items;

-- Drop the function (if it exists)
DROP FUNCTION IF EXISTS update_updated_at_column ();
