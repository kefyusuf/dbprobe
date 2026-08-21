CREATE DATABASE IF NOT EXISTS shop;

CREATE TABLE IF NOT EXISTS shop.customers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_customers_email (email),
    KEY idx_customers_name (name)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS shop.orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL,
    total_cents BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_orders_customer (customer_id),
    KEY idx_orders_customer_created (customer_id, created_at),
    KEY idx_orders_status (status),
    CONSTRAINT chk_orders_total_nonnegative CHECK (total_cents >= 0),
    CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES shop.customers(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
) ENGINE=InnoDB;

INSERT INTO shop.customers (email, name)
VALUES
    ('alice@example.test', 'Alice'),
    ('bob@example.test', 'Bob'),
    ('carol@example.test', 'Carol');

INSERT INTO shop.orders (customer_id, status, total_cents)
VALUES
    (1, 'paid', 1200),
    (1, 'paid', 2200),
    (2, 'pending', 3400),
    (3, 'cancelled', 900);

CREATE USER IF NOT EXISTS 'dbprobe'@'%' IDENTIFIED BY 'dbprobe-pass';
GRANT SELECT ON shop.* TO 'dbprobe'@'%';
GRANT SELECT ON performance_schema.* TO 'dbprobe'@'%';
GRANT SELECT ON sys.* TO 'dbprobe'@'%';
GRANT PROCESS ON *.* TO 'dbprobe'@'%';
