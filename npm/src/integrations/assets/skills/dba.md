---
name: trackfw-dba-skill
description: Database modeling, query optimization, high availability and vector data patterns.
---

## Modeling

Choose the data model based on access patterns, not familiarity:
- **Relational (SQL)**: normalized schemas, foreign key integrity, ACID transactions. Best when
  relationships are complex and consistency is paramount.
- **Document**: flexible schema for hierarchical, polymorphic data. Beware of embedding without
  considering update patterns.
- **Key-Value / Wide-Column**: for high-throughput reads/writes with simple access patterns.
  Design the key structure before creating the table — it cannot be changed cheaply.
- **Graph**: for highly connected data where traversal depth and relationship types vary.

Model for queries, not for purity. A schema that cannot be queried efficiently without
full-table scans has the wrong design.

## Indexing

- Create indexes to support the actual query patterns in the application; unused indexes
  consume write overhead and space.
- Composite indexes: column order matters. The most selective column first, then the columns
  used in equality filters, then range predicates.
- Partial indexes (filtered indexes) reduce index size and improve write throughput when a
  significant fraction of rows never appear in queries.
- Validate index usage with execution plans (`EXPLAIN ANALYZE`); an index that the optimizer
  ignores is waste.

## Query Optimization

- Read the execution plan for every slow query; identify table scans, nested loops and
  sort operations that could be eliminated.
- Avoid `SELECT *`; project only the columns the application uses.
- Parameterize queries; never concatenate user input into SQL strings.
- Push filters as early as possible in query pipelines (aggregations, joins).
- Analyze and update table statistics regularly; stale statistics cause bad query plans.

## High Availability and Replication

- Define RTO and RPO per service tier before choosing an HA architecture.
- Read replicas for read-heavy workloads; route analytics queries away from the primary.
- Point-in-time recovery (PITR) is mandatory for production databases.
- Test backup restoration on a schedule; a backup that has never been restored is
  unverified and therefore unreliable.
- Cross-region replication for disaster recovery; understand replication lag and its
  impact on read-after-write consistency.

## Cache Integration

- **Cache-aside**: application reads from cache; on miss, reads from database and populates
  cache. Simple and explicit. Handles cache invalidation at write time.
- **Write-through**: write to both cache and database atomically. Keeps cache consistent at
  the cost of write latency.
- Set TTLs on every cache entry; unbounded cache growth leads to eviction unpredictability.
- Cache only safe data (no PII without appropriate controls); audit what is cached.

## Vector Databases

For AI/ML retrieval workloads:
- **Index choice**: HNSW (Hierarchical Navigable Small World) for low-latency ANN search;
  IVFFlat for higher-recall bulk workloads.
- **Hybrid search**: combine dense vector search with sparse (BM25) text search using
  Reciprocal Rank Fusion (RRF). Pure dense search underperforms on keyword-heavy queries.
- **Reranking**: apply a cross-encoder reranker after initial retrieval to improve top-k
  precision without the cost of reranking the full corpus.
- Match the embedding model dimensionality to the index; switching models requires
  re-embedding the entire corpus.

## Analytics and Columnar Storage

- Columnar engines (ClickHouse, DuckDB, BigQuery) are optimized for aggregations over a
  subset of columns in large tables; do not use them for OLTP row-level operations.
- Materialized views pre-aggregate expensive queries; refresh strategy (incremental vs full)
  depends on freshness requirements and write volume.
- Partition large tables by time or a high-cardinality dimension to enable partition pruning.
