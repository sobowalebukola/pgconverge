# About pgconverge

**pgconverge** is a powerful tool for converting JSON-defined database schemas into deterministic SQL DDL statements, with full support for **multidirectional replication**. It allows developers and teams to manage database schema changes in a structured, predictable, and reversible way.

## Key Features

* **JSON to SQL DDL**: Define your database schema in JSON and generate ready-to-run SQL statements.
* **Bidirectional Schema Support**: Designed to handle multi-node database replication scenarios.
* **Deterministic Output**: Ensures consistent and predictable SQL DDL, making schema diffs and migrations safer.
* **Schema Diffing**: Detect changes between JSON schemas and generate SQL to apply or revert updates.
* **Multi-Database Ready**: Works with PostgreSQL and easily extendable to other relational databases.

## Why pgconverge?

Managing database schemas across distributed systems can be error-prone. pgconverge makes it easier to:

* Keep your database schemas in version control.
* Automate schema migrations and replication-safe updates.
* Collaborate on schema design without manual SQL scripts.

## Getting Started

1. Define your schema in JSON.
2. Run pgconverge to generate SQL DDL statements.
3. Apply the generated SQL to your database nodes.
