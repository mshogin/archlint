# System Architecture Audit: [System Name]

**EN** | [RU](system-audit.md)

---

## Executive Summary

| Area                | Score  | Status                   |
|---------------------|--------|--------------------------|
| System Structure    | [1-5]  | [OK/Needs Attention/Critical] |
| Service Interaction | [1-5]  | [OK/Needs Attention/Critical] |
| Patterns & Practices| [1-5]  | [OK/Needs Attention/Critical] |
| Test Coverage       | [1-5]  | [OK/Needs Attention/Critical] |
| Observability       | [1-5]  | [OK/Needs Attention/Critical] |
| Resilience          | [1-5]  | [OK/Needs Attention/Critical] |

**Overall Score:** [X]/5

**Critical Issues:** [X]
**Improvement Recommendations:** [X]

---

## 1. System Overview

### 1.1 Purpose and Boundaries

| Parameter           | Value                                 |
|---------------------|---------------------------------------|
| Business Domain     | [Description]                         |
| System Purpose      | [Description]                         |
| Number of Services  | [X]                                   |
| Architectural Style | [Microservices/Modular Monolith/etc.] |
| System Age          | [X years/months]                      |

### 1.2 Context Diagram (C4 Level 1)

```
[Insert diagram or description]

External Systems:
- [System A] - [purpose, integration type]
- [System B] - [purpose, integration type]

Users:
- [User type 1] - [count, usage pattern]
- [User type 2] - [count, usage pattern]
```

---

## 2. System Structure

### 2.1 Service Catalog

| # | Service     | Purpose       | Technologies     | Owner    | Criticality      |
|---|-------------|---------------|------------------|----------|------------------|
| 1 | [service-1] | [Description] | [Go, PostgreSQL] | [Team]   | [High/Medium/Low]|
| 2 | [service-2] | [Description] | [Go, Kafka]      | [Team]   | [High/Medium/Low]|
| 3 | [service-3] | [Description] | [Python, Redis]  | [Team]   | [High/Medium/Low]|

### 2.2 Container Diagram (C4 Level 2)

```
[Insert diagram or description of service relationships]
```

### 2.3 Bounded Contexts (if DDD)

| Context     | Services               | Core/Supporting/Generic |
|-------------|------------------------|-------------------------|
| [Context 1] | [service-1, service-2] | [Core Domain]           |
| [Context 2] | [service-3]            | [Supporting]            |
| [Context 3] | [service-4]            | [Generic]               |

### 2.4 Structure Assessment

- [ ] Services have clear responsibility boundaries
- [ ] No functionality duplication between services
- [ ] Service sizes are adequate (not too large, not too small)
- [ ] Clear layer separation (API Gateway, BFF, Backend, Data)

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 3. Service Interaction

### 3.1 Synchronous Interactions

| Source      | Target      | Protocol    | Purpose       | SLA    |
|-------------|-------------|-------------|---------------|--------|
| [service-1] | [service-2] | [REST/gRPC] | [Description] | [X ms] |
| [service-2] | [service-3] | [REST/gRPC] | [Description] | [X ms] |

**Assessment:**
- [ ] Timeouts configured correctly
- [ ] Retry with exponential backoff
- [ ] Circuit breaker present
- [ ] No synchronous chains > 3 services

### 3.2 Asynchronous Interactions

| Publisher   | Topic/Queue | Consumer               | Broker  | Delivery Guarantee |
|-------------|-------------|------------------------|---------|--------------------|
| [service-1] | [topic-1]   | [service-2]            | [Kafka] | [At-least-once]    |
| [service-2] | [topic-2]   | [service-3, service-4] | [Kafka] | [At-least-once]    |

**Assessment:**
- [ ] Consumers are idempotent
- [ ] DLQ for failed messages
- [ ] Lag monitoring configured
- [ ] Poison message handling strategy

### 3.3 Dependency Matrix

|       | svc-1 | svc-2 | svc-3 | svc-4 | svc-5 |
|-------|-------|-------|-------|-------|-------|
| svc-1 | -     | S     | A     | -     | -     |
| svc-2 | -     | -     | S     | A     | -     |
| svc-3 | -     | -     | -     | -     | S     |
| svc-4 | -     | -     | -     | -     | A     |
| svc-5 | -     | -     | -     | -     | -     |

*S = Sync, A = Async*

### 3.4 Critical Paths

| # | Path                                      | Latency (p99) | Risk          |
|---|-------------------------------------------|---------------|---------------|
| 1 | [User -> API GW -> svc-1 -> svc-2 -> DB]  | [X ms]        | [Description] |
| 2 | [Event -> svc-3 -> svc-4 -> External API] | [X ms]        | [Description] |

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 4. Patterns and Practices

### 4.1 Used Patterns

| Pattern         | Where Used         | Implementation               | Assessment      |
|-----------------|--------------------|-----------------------------|-----------------|
| API Gateway     | [service-gateway]  | [Kong/Custom]               | [OK/Partial/No] |
| Circuit Breaker | [service-1, service-2] | [hystrix/resilience4j]  | [OK/Partial/No] |
| Saga            | [order-service]    | [Choreography/Orchestration]| [OK/Partial/No] |
| CQRS            | [reporting-service]| [Description]               | [OK/Partial/No] |
| Event Sourcing  | [audit-service]    | [Description]               | [OK/Partial/No] |
| Outbox Pattern  | [service-1]        | [Debezium/Custom]           | [OK/Partial/No] |
| Sidecar         | [all services]     | [Envoy/Linkerd]             | [OK/Partial/No] |

### 4.2 Architecture Decision Records (ADR)

| ADR     | Name                        | Status     | Context |
|---------|-----------------------------|------------|---------|
| ADR-001 | [Message broker selection]  | [Accepted] | [Link]  |
| ADR-002 | [Authentication strategy]   | [Accepted] | [Link]  |
| ADR-003 | [Caching approach]          | [Accepted] | [Link]  |

**Assessment:**
- [ ] ADRs documented for significant decisions
- [ ] ADRs contain context, decision, consequences
- [ ] Deprecated ADRs marked and link to replacement

### 4.3 Anti-patterns

| # | Anti-pattern         | Where Found                          | Impact | Recommendation       |
|---|----------------------|--------------------------------------|--------|----------------------|
| 1 | Distributed Monolith | [service-1, service-2 tightly coupled] | [High] | [Refactor boundaries]|
| 2 | Sync over Async      | [service-3 blocking wait on Kafka]   | [Medium] | [Redesign to async]|
| 3 | Shared Database      | [service-4, service-5 same DB]       | [High] | [Split database]     |

---

## 5. Data and Storage

### 5.1 Databases

| Service     | DB           | Type       | Size   | Redundancy       |
|-------------|--------------|------------|--------|------------------|
| [service-1] | [PostgreSQL] | [OLTP]     | [X GB] | [Master-Replica] |
| [service-2] | [MongoDB]    | [Document] | [X GB] | [ReplicaSet]     |
| [service-3] | [ClickHouse] | [OLAP]     | [X GB] | [Cluster]        |

### 5.2 Caching

| Service     | Cache       | Strategy        | TTL     | Hit Rate |
|-------------|-------------|-----------------|---------|----------|
| [service-1] | [Redis]     | [Cache-aside]   | [X min] | [X%]     |
| [service-2] | [Memcached] | [Write-through] | [X min] | [X%]     |

### 5.3 Data Consistency

| Scenario            | Consistency Level | Implementation |
|---------------------|-------------------|----------------|
| [Order creation]    | [Strong]          | [2PC/Saga]     |
| [Catalog update]    | [Eventual]        | [Events]       |
| [Analytics]         | [Eventual]        | [CDC -> DWH]   |

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 6. Test Coverage

### 6.1 Coverage by Service

| Service     | Unit | Integration | E2E  | Contract | Overall |
|-------------|------|-------------|------|----------|---------|
| [service-1] | [X%] | [X%]        | [X%] | [Yes/No] | [X%]    |
| [service-2] | [X%] | [X%]        | [X%] | [Yes/No] | [X%]    |
| [service-3] | [X%] | [X%]        | [X%] | [Yes/No] | [X%]    |

**Standard:** Unit >= 70%, Integration >= 50%, E2E - critical paths

### 6.2 Contract Testing

- [ ] Contract testing used (Pact, etc.)
- [ ] Consumer-driven contracts
- [ ] Contracts versioned
- [ ] CI contract verification

### 6.3 Load Testing

| Scenario      | Tool         | Last Run     | Result              |
|---------------|--------------|--------------|---------------------|
| [Peak load]   | [k6/Gatling] | [YYYY-MM-DD] | [X RPS, Y ms p99]   |
| [Stress test] | [k6/Gatling] | [YYYY-MM-DD] | [Limit X RPS]       |
| [Soak test]   | [k6/Gatling] | [YYYY-MM-DD] | [Stable X hours]    |

### 6.4 Chaos Engineering

- [ ] Chaos experiments conducted
- [ ] Game days held
- [ ] Known failure points documented

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 7. Observability

### 7.1 Metrics

| Level          | Tool                | Coverage | Retention |
|----------------|---------------------|----------|-----------|
| Infrastructure | [Prometheus]        | [X%]     | [X days]  |
| Application    | [Prometheus]        | [X%]     | [X days]  |
| Business       | [Custom/Prometheus] | [X%]     | [X days]  |

**Key Metrics:**
- [ ] RED metrics (Rate, Errors, Duration) for all services
- [ ] USE metrics (Utilization, Saturation, Errors) for infrastructure
- [ ] Business metrics (conversion, orders, etc.)

### 7.2 Logging

| Parameter                | Value             |
|--------------------------|-------------------|
| Centralized Logging      | [ELK/Loki/etc.]   |
| Log Format               | [JSON/Structured] |
| Correlation ID           | [Yes/No]          |
| Retention                | [X days]          |

**Assessment:**
- [ ] Unified log format across all services
- [ ] Correlation/trace ID present
- [ ] Logs don't contain sensitive data
- [ ] Log levels configured

### 7.3 Tracing

| Parameter           | Value                 |
|---------------------|-----------------------|
| Distributed Tracing | [Jaeger/Tempo/Zipkin] |
| Service Coverage    | [X%]                  |
| Sampling Rate       | [X%]                  |

**Assessment:**
- [ ] Traces linked across services
- [ ] Spans contain useful information
- [ ] Alerts on anomalous traces

### 7.4 Alerting

| Category             | Alert Count | Coverage      |
|----------------------|-------------|---------------|
| Critical (PagerDuty) | [X]         | [Description] |
| Warning (Slack)      | [X]         | [Description] |
| Info                 | [X]         | [Description] |

**Assessment:**
- [ ] Alerts are actionable (clear what to do)
- [ ] Runbooks for critical alerts
- [ ] No alert fatigue (< 5 critical/day normal)
- [ ] Escalation policies configured

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 8. Resilience and Fault Tolerance

### 8.1 Single Points of Failure

| # | Component     | Risk                   | Mitigation          |
|---|---------------|------------------------|---------------------|
| 1 | [Description] | [Critical/High/Medium] | [Current/Planned]   |
| 2 | [Description] | [Critical/High/Medium] | [Current/Planned]   |

### 8.2 Graceful Degradation

| Failure Scenario    | System Behavior              | Assessment |
|---------------------|------------------------------|------------|
| [DB unavailable]    | [Fallback to cache / Error]  | [OK/No]    |
| [External API down] | [Circuit breaker / Retry]    | [OK/No]    |
| [Kafka unavailable] | [Local queue / Error]        | [OK/No]    |

### 8.3 Scalability

| Service     | Current Load | Limit   | Bottleneck       |
|-------------|--------------|---------|------------------|
| [service-1] | [X RPS]      | [Y RPS] | [DB connections] |
| [service-2] | [X RPS]      | [Y RPS] | [CPU]            |

### 8.4 Disaster Recovery

| Parameter         | Requirement | Current      | Status   |
|-------------------|-------------|--------------|----------|
| RTO               | [X hours]   | [Y hours]    | [OK/NOK] |
| RPO               | [X hours]   | [Y hours]    | [OK/NOK] |
| Last DR Test      | -           | [YYYY-MM-DD] | [OK/NOK] |

**Issues:**
| # | Issue         | Impact            | Recommendation |
|---|---------------|-------------------|----------------|
| 1 | [Description] | [High/Medium/Low] | [What to do]   |

---

## 9. Security

### 9.1 Authentication and Authorization

| Aspect                  | Implementation       | Assessment      |
|-------------------------|----------------------|-----------------|
| Service-to-service auth | [mTLS/JWT/API Keys]  | [OK/Partial/No] |
| User authentication     | [OAuth2/OIDC/Custom] | [OK/Partial/No] |
| Authorization           | [RBAC/ABAC/Custom]   | [OK/Partial/No] |

### 9.2 Data Protection

- [ ] Encryption at rest
- [ ] Encryption in transit (TLS everywhere)
- [ ] PII masking in logs
- [ ] Secrets in Vault/k8s secrets (not in code)

### 9.3 Known Vulnerabilities

| # | Vulnerability | Service     | Severity               | Status       |
|---|---------------|-------------|------------------------|--------------|
| 1 | [Description] | [service-1] | [Critical/High/Medium] | [Open/Fixed] |

---

## 10. Issues and Recommendations Summary

### 10.1 Critical Issues

| # | Issue         | Area                          | Recommendation | Priority |
|---|---------------|-------------------------------|----------------|----------|
| 1 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P0       |
| 2 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P0       |

### 10.2 Important Improvements

| # | Issue         | Area                          | Recommendation | Priority |
|---|---------------|-------------------------------|----------------|----------|
| 1 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P1       |
| 2 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P1       |

### 10.3 Desirable Improvements

| # | Issue         | Area                          | Recommendation | Priority |
|---|---------------|-------------------------------|----------------|----------|
| 1 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P2       |
| 2 | [Description] | [Structure/Interaction/etc.]  | [What to do]   | P2       |

---

## 11. Improvement Plan

### Immediate Actions (1-2 weeks)

- [ ] [Action 1] - owner: [Name]
- [ ] [Action 2] - owner: [Name]

### Medium-term (1-3 months)

- [ ] [Action 1] - owner: [Name]
- [ ] [Action 2] - owner: [Name]

### Long-term (3+ months)

- [ ] [Action 1] - owner: [Name]
- [ ] [Action 2] - owner: [Name]

---

## 12. Conclusion

**Overall System Score:** [X]/5

| Criterion                | Score  | Comment       |
|--------------------------|--------|---------------|
| Architecture Maturity    | [1-5]  | [Comment]     |
| Operational Readiness    | [1-5]  | [Comment]     |
| Fault Resilience         | [1-5]  | [Comment]     |
| Scalability              | [1-5]  | [Comment]     |
| Security                 | [1-5]  | [Comment]     |

**Key Findings:**
1. [Finding 1]
2. [Finding 2]
3. [Finding 3]

**Next Audit Recommended:** [in X months]

---

*Audit Conducted: [YYYY-MM-DD]*
