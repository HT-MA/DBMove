CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id),
    amount NUMERIC(12, 2) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO users (id, email, name) VALUES
    (1, 'alice@example.com', 'Alice'),
    (2, 'bob@example.com', 'Bob'),
    (3, 'carol@example.com', 'Carol');

INSERT INTO orders (id, user_id, amount, status) VALUES
    (1, 1, 199.90, 'paid'),
    (2, 1, 59.00, 'pending'),
    (3, 2, 399.00, 'shipped'),
    (4, 3, 1299.50, 'paid'),
    (5, 2, 20.00, 'cancelled');

-- second database used for multi-database migration verification
CREATE DATABASE analytics_db;
\connect analytics_db

CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    payload TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO events (id, event_type, payload) VALUES
    (1, 'page_view', '{"page": "/dashboard"}'),
    (2, 'click', '{"button": "start"}');
