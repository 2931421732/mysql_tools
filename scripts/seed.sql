SET SESSION cte_max_recursion_depth = 100000;
DROP DATABASE IF EXISTS demo_shop;
CREATE DATABASE demo_shop CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
USE demo_shop;

CREATE TABLE customers (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(80) NOT NULL,
  email VARCHAR(160) NOT NULL,
  country CHAR(2) NULL,
  vip TINYINT(1) NOT NULL DEFAULT 0,
  balance DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  profile JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_customers_email (email),
  KEY idx_customers_country (country)
) ENGINE=InnoDB COMMENT='客户主表';

CREATE TABLE products (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  sku VARCHAR(40) NOT NULL,
  title VARCHAR(120) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  stock INT NOT NULL DEFAULT 0,
  active TINYINT(1) NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  UNIQUE KEY uk_products_sku (sku)
) ENGINE=InnoDB COMMENT='商品';

CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  customer_id INT UNSIGNED NOT NULL,
  status ENUM('new','paid','shipped','cancelled') NOT NULL DEFAULT 'new',
  total DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  note TEXT NULL,
  placed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_orders_customer (customer_id),
  KEY idx_orders_status (status),
  CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB COMMENT='订单';

CREATE TABLE order_items (
  order_id BIGINT UNSIGNED NOT NULL,
  product_id INT UNSIGNED NOT NULL,
  qty INT NOT NULL DEFAULT 1,
  unit_price DECIMAL(10,2) NOT NULL,
  PRIMARY KEY (order_id, product_id),
  KEY idx_items_product (product_id),
  CONSTRAINT fk_items_order FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE CASCADE,
  CONSTRAINT fk_items_product FOREIGN KEY (product_id) REFERENCES products (id)
) ENGINE=InnoDB;

INSERT INTO customers (name,email,country,vip,balance,profile) VALUES
 ('张伟','zhangwei@example.com','CN',1,1280.50,'{"tier":"gold","tags":["vip","early"]}'),
 ('李娜','lina@example.com','CN',0,0.00,NULL),
 ('Alice Chen','alice@example.com','SG',1,4200.00,'{"tier":"platinum"}'),
 ('Bob Smith','bob@example.com','US',0,320.75,'{"tier":"basic","tags":[]}'),
 ('山田太郎','yamada@example.com','JP',1,980.00,NULL),
 ('Chloé Martin','chloe@example.com','FR',0,150.25,'{"tier":"gold"}');

INSERT INTO products (sku,title,price,stock,active) VALUES
 ('SKU-1001','机械键盘 87 键',399.00,42,1),
 ('SKU-1002','人体工学椅',1899.00,7,1),
 ('SKU-1003','4K 显示器 27"',2399.00,12,1),
 ('SKU-2001','USB-C Hub',259.00,120,1),
 ('SKU-2002','降噪耳机',1299.00,0,0);

INSERT INTO orders (customer_id,status,total,note,placed_at) VALUES
 (1,'paid',399.00,'首单',NOW() - INTERVAL 9 DAY),
 (1,'shipped',2399.00,NULL,NOW() - INTERVAL 7 DAY),
 (3,'paid',1899.00,'请开发票',NOW() - INTERVAL 5 DAY),
 (4,'new',518.00,NULL,NOW() - INTERVAL 2 DAY),
 (5,'cancelled',1299.00,'缺货取消',NOW() - INTERVAL 12 DAY),
 (6,'paid',399.00,'gift wrap',NOW() - INTERVAL 1 DAY);

INSERT INTO order_items (order_id,product_id,qty,unit_price) VALUES
 (1,1,1,399.00),(2,3,1,2399.00),(3,2,1,1899.00),(4,4,2,259.00),(5,5,1,1299.00),(6,1,1,399.00);

CREATE OR REPLACE VIEW v_order_summary AS
SELECT o.id, c.name AS customer, o.status, o.total, o.placed_at
FROM orders o JOIN customers c ON c.id = o.customer_id;

CREATE TABLE big_table (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payload VARCHAR(200) NOT NULL,
  n INT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB;

INSERT INTO big_table (payload,n)
SELECT CONCAT('row-', seq), seq % 97
FROM (WITH RECURSIVE s(seq) AS (SELECT 1 UNION ALL SELECT seq+1 FROM s WHERE seq < 2500) SELECT seq FROM s) t;

DROP DATABASE IF EXISTS analytics;
CREATE DATABASE analytics CHARACTER SET utf8mb4;
CREATE TABLE analytics.events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  type VARCHAR(40) NOT NULL,
  happened_at DATETIME NOT NULL,
  payload JSON NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB;
INSERT INTO analytics.events (type,happened_at,payload) VALUES
 ('signup',NOW(),'{"source":"web"}'),('purchase',NOW(),'{"amount":99}'),('signup',NOW(),NULL);

DROP USER IF EXISTS 'navmini'@'%';
CREATE USER 'navmini'@'%' IDENTIFIED BY 'navmini_pw';
GRANT ALL PRIVILEGES ON demo_shop.* TO 'navmini'@'%';
GRANT ALL PRIVILEGES ON analytics.* TO 'navmini'@'%';
GRANT PROCESS, SELECT ON *.* TO 'navmini'@'%';
FLUSH PRIVILEGES;
