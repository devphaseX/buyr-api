BEGIN

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;


CREATE TRIGGER update_audits_updated_at
BEFORE UPDATE ON audits
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_whitelists_updated_at
BEFORE UPDATE ON whitelists
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_category_updated_at
BEFORE UPDATE ON category
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_product_features_updated_at
BEFORE UPDATE ON product_features
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_vendor_users_updated_at
BEFORE UPDATE ON vendor_users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_notifications_updated_at
BEFORE UPDATE ON notifications
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reviews_updated_at
BEFORE UPDATE ON reviews
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_carts_updated_at
BEFORE UPDATE ON carts
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_admin_users_updated_at
BEFORE UPDATE ON admin_users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_product_images_updated_at
BEFORE UPDATE ON product_images
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_order_items_updated_at
BEFORE UPDATE ON order_items
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_normal_users_updated_at
BEFORE UPDATE ON normal_users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payments_updated_at
BEFORE UPDATE ON payments
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_addresses_updated_at
BEFORE UPDATE ON addresses
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER update_cart_items_updated_at
BEFORE UPDATE ON cart_items
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMIT
