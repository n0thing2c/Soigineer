CREATE DATABASE IF NOT EXISTS logs_db;

USE logs_db;

CREATE TABLE IF NOT EXISTS logs_table (
    EventID            String,
    ApplicationName    LowCardinality(String),
    Level              LowCardinality(String),
    Message            String,
    NormalizedMessage  String,
    Timestamp          DateTime64(3, 'UTC'),
    ReceivedAt         DateTime64(3, 'UTC'),
    TraceID            String,
    Fingerprint        String
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(Timestamp)
ORDER BY (EventID, ApplicationName, Level, Timestamp, TraceID)
TTL toDateTime(Timestamp) + INTERVAL 30 DAY;
