# Multiple alias rules per resource

A source resource may have multiple alias rules, including rules whose metadata conditions could match the same node. Rules are evaluated in persisted configuration order; the first matching rule supplies the alias and display unit.

This supersedes ADR 0005's conservative conflict rejection. It allows operators to express multiple hardware variants for one Kubernetes resource while retaining deterministic accounting for overlapping rules.
