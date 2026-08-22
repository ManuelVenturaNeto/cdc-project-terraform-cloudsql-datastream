CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS addresses (
    id SERIAL PRIMARY KEY,
    street TEXT NOT NULL,
    number TEXT NOT NULL,
    complement TEXT,
    district TEXT,
    city TEXT NOT NULL,
    state TEXT NOT NULL,
    zip_code TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'BR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_addresses (
    user_id INT NOT NULL REFERENCES users (id),
    address_id INT NOT NULL REFERENCES addresses (id),
    label TEXT NOT NULL DEFAULT 'home',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, address_id)
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'placed',
    total_amount NUMERIC(10, 2) NOT NULL,
    shipping_address_id INT NOT NULL REFERENCES addresses (id),
    placed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_orders (
    user_id INT NOT NULL REFERENCES users (id),
    order_id INT NOT NULL REFERENCES orders (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, order_id)
);

CREATE INDEX IF NOT EXISTS user_addresses_address_id_idx ON user_addresses (address_id);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);
CREATE INDEX IF NOT EXISTS orders_shipping_address_id_idx ON orders (shipping_address_id);
CREATE INDEX IF NOT EXISTS user_orders_order_id_idx ON user_orders (order_id);
