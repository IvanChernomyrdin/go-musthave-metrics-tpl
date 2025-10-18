-- CREATE TABLE IF NOT EXISTS metrics (
--     id SERIAL PRIMARY KEY,
--     mtype VARCHAR(10) NOT NULL,
--     name VARCHAR(255) NOT NULL,
--     gauge_value DOUBLE PRECISION,
--     delta_value BIGINT,
--     UNIQUE (mtype, name)
-- );

-- CREATE INDEX IF NOT EXISTS idx_metrics_type_name ON metrics (mtype, name);


CREATE TABLE IF NOT EXISTS metrics (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    mtype VARCHAR(50) NOT NULL,
    delta BIGINT,
    value DOUBLE PRECISION,
    hash VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_id ON metrics (id);
CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (mtype);