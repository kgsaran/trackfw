---
name: trackfw-data-skill
description: Data pipelines, quality contracts, lakehouse patterns and MLOps.
---

## Pipeline Design Principles

- **Idempotent**: running a pipeline twice with the same inputs produces the same outputs with
  no side effects. Idempotency enables safe retries and backfills.
- **Observable**: every pipeline run emits metrics (rows processed, duration, error count)
  and structured logs. Silent pipelines hide data quality problems.
- **Incremental by default**: process only new or changed data; full reloads are expensive and
  error-prone. Implement watermark or change-data-capture (CDC) patterns.
- **Testable**: test transformations with deterministic fixtures. A pipeline that cannot be
  run locally with sample data is hard to debug.

## Data Contracts

A data contract is a versioned schema agreement between a data producer and its consumers.
Breaking schema changes require a new contract version and a migration window:
- Define contracts before building pipelines, not after.
- Validate incoming data against the contract on ingestion; reject or quarantine violations.
- Version schemas explicitly (SemVer or a sequential version field).
- Treat a schema change that breaks downstream consumers as a breaking change — the same way
  an API breaking change is treated.

## ELT / Transformation

- Prefer ELT (Extract, Load, Transform) over ETL when the target system can handle the
  transformation cost; push compute to where the data lives.
- SQL transformations with dbt are version-controlled, testable and self-documenting.
- Model data in layers: raw (landing, immutable), staged (typed, deduplicated), marts
  (business-aligned aggregations). Never query raw data directly in reports.
- Materialized models for expensive aggregations; incremental models for large tables.

## Streaming and Real-Time

- Use a durable, partitioned log (Kafka, Pulsar) for event streaming; producers and consumers
  are decoupled in time and scale independently.
- Guarantee semantics: at-least-once is the safe default; exactly-once requires coordinated
  transactions between the broker and the sink.
- Windowing functions (tumbling, sliding, session) for time-based aggregations in stream
  processing.
- Monitor consumer lag continuously; unprocessed lag accumulation is an early warning of
  a performance or capacity problem.

## Data Quality

- Data quality checks belong in the pipeline, not in the dashboard. Fail fast, fail clearly.
- Test categories: **schema** (types, nullability, column existence), **integrity** (referential
  keys, uniqueness), **freshness** (last-updated within expected window), **statistical**
  (distribution drift, unexpected nulls or zeros).
- Quarantine rows that fail quality checks rather than silently dropping them; dropped rows
  hide data completeness problems.
- SLA: define freshness and completeness SLAs per dataset; alert on SLA breach, not on
  arbitrary thresholds.

## Lakehouse and Storage

- Store raw data in an open, columnar format (Parquet) in object storage; this decouples
  compute from storage and avoids vendor lock-in.
- Table formats (Delta Lake, Apache Iceberg) add ACID transactions, schema evolution and
  time travel to object storage. Use them for tables that require updates or deletes.
- Partition large tables by a time dimension and a high-cardinality business key to enable
  efficient pruning.

## MLOps

- Feature stores decouple feature engineering from model training; online serving and offline
  training use the same feature definitions (point-in-time correctness).
- Track every experiment: hyperparameters, dataset version, metrics, artifact location.
  Reproducibility requires logging, not memory.
- Model deployment is a software deployment: versioned, tested, monitored, rollback-capable.
- Monitor model performance in production: data drift, prediction drift, and business metric
  correlation. Retraining triggers are defined by drift thresholds, not by calendar.
