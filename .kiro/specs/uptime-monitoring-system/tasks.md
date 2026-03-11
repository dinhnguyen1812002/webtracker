# Implementation Plan: Uptime Monitoring & Alert System

## Overview

This implementation plan breaks down the Uptime Monitoring & Alert System into discrete, incremental coding tasks. The approach follows Clean Architecture principles, building from the domain layer outward. Each task builds on previous work, with property-based tests integrated throughout to validate correctness early.

The implementation prioritizes core monitoring functionality first, then adds alerting, metrics, and finally the dashboard. Testing tasks are marked as optional to enable faster MVP delivery while maintaining the option for comprehensive testing.

## Tasks

- [x] 1. Project setup and domain layer
  - Initialize Go module with required dependencies (gin, pgx, redis, gopter, templ)
  - Create directory structure following Clean Architecture (domain, usecase, interface, infrastructure)
  - Define domain entities (Monitor, HealthCheck, Alert) with validation methods
  - Define repository interfaces in domain layer
  - _Requirements: 10.1, 10.6_

- [x] 1.1 Write property tests for domain entities
  - **Property 8: Check Interval Validation**
  - **Property 12: URL Validation**
  - **Validates: Requirements 3.1, 3.2, 10.1**

- [x] 2. Database infrastructure layer
  - [x] 2.1 Implement PostgreSQL repository for monitors
    - Create connection pool with pgx
    - Implement MonitorRepository interface (Create, GetByID, List, Update, Delete)
    - Add database migrations for monitors table
    - _Requirements: 10.6_
  
  - [x] 2.2 Implement PostgreSQL repository for health checks
    - Implement HealthCheckRepository interface
    - Add database migrations for health_checks table
    - Add indexes for query performance
    - _Requirements: 14.1, 14.3_
  
  - [x] 2.3 Implement PostgreSQL repository for alerts
    - Implement AlertRepository interface
    - Add database migrations for alerts table
    - Add indexes for query performance
    - _Requirements: 13.1, 13.3_

- [ ]* 2.4 Write property tests for repositories
  - **Property 13: Configuration Persistence**
  - **Property 41: Query Filtering Support**
  - **Validates: Requirements 10.6, 13.3, 14.3**

- [x] 3. Redis infrastructure layer
  - [x] 3.1 Implement Redis client wrapper
    - Create connection pool with redis-go
    - Implement cache operations (Get, Set, Delete with TTL)
    - Implement job queue operations (Enqueue, Dequeue, Schedule)
    - _Requirements: 12.5_
  
  - [x] 3.2 Implement monitor configuration caching
    - Cache monitor configs with 5-minute TTL
    - Implement cache invalidation on updates
    - _Requirements: 12.5_

- [ ]* 3.3 Write property tests for caching
  - **Property 50: Monitor Configuration Caching**
  - **Validates: Requirements 12.5**

- [x] 4. HTTP client infrastructure
  - [x] 4.1 Implement HTTP client with connection pooling
    - Configure connection pool (MaxIdleConns=100, MaxIdleConnsPerHost=10)
    - Set 30-second timeout
    - Configure TLS to capture certificate details
    - _Requirements: 1.1, 1.5, 12.4_
  
  - [x] 4.2 Implement retry logic with exponential backoff
    - Retry failed requests up to 3 times total
    - Use exponential backoff (1s, 2s, 4s)
    - _Requirements: 1.6_

- [ ]* 4.3 Write property tests for HTTP client
  - **Property 1: HTTP Request Execution**
  - **Property 4: Retry with Exponential Backoff**
  - **Property 49: HTTP Connection Reuse**
  - **Validates: Requirements 1.1, 1.6, 12.4**

- [ ] 5. Health check service implementation
  - [x] 5.1 Implement core health check execution logic
    - Execute HTTP/HTTPS requests
    - Measure response time
    - Classify status codes (200-299 = success, others = failure)
    - Handle timeouts and network errors
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_
  
  - [x] 5.2 Implement SSL certificate extraction and validation
    - Extract SSL certificate from HTTPS responses
    - Calculate days until expiration
    - Validate certificate chain
    - _Requirements: 2.1, 2.6_
  
  - [x] 5.3 Implement health check persistence
    - Save health check results to database
    - Update metrics cache in Redis
    - _Requirements: 14.1_


- [x] 6. Checkpoint - Core health checking functional
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. Alert service implementation
  - [x] 7.1 Implement alert generation logic
    - Generate alerts for downtime, recovery, SSL expiration
    - Determine alert severity based on conditions
    - Persist alerts to database
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 5.1_
  
  - [x] 7.2 Implement rate limiting for alerts
    - 15-minute suppression for duplicate downtime alerts
    - 1-hour reminder for prolonged failures
    - Daily limit for SSL expiration alerts
    - Exception for recovery alerts
    - _Requirements: 5.2, 5.3, 5.4, 5.5_


- [x] 8. Alert channel adapters
  - [x] 8.1 Implement Telegram adapter
    - Send formatted messages via Telegram Bot API
    - Implement retry logic with exponential backoff (3 attempts)
    - _Requirements: 4.2, 4.5_
  
  - [x] 8.2 Implement Email adapter
    - Send emails via SMTP with TLS
    - Use HTML email templates
    - Implement retry logic with exponential backoff (3 attempts)
    - _Requirements: 4.3, 4.5_
  
  - [x] 8.3 Implement Webhook adapter
    - Send HTTP POST with JSON payload
    - Support custom headers
    - Implement retry logic with exponential backoff (3 attempts)
    - _Requirements: 4.4, 4.5_
  
  - [x] 8.4 Implement multi-channel alert delivery
    - Deliver alerts to all configured channels concurrently
    - Isolate channel failures (continue with other channels)
    - Log delivery failures after max retries
    - _Requirements: 4.1, 4.6_


- [x] 9. Worker pool and job scheduler
  - [x] 9.1 Implement worker pool
    - Create configurable number of worker goroutines (default 10)
    - Workers consume jobs from Redis queue
    - Implement graceful shutdown
    - Track worker pool statistics (active workers, queue depth)
    - _Requirements: 6.1, 6.2, 6.3, 6.4_
  
  - [x] 9.2 Implement job scheduler
    - Use Redis sorted sets for time-based scheduling
    - Enqueue jobs when scheduled time arrives
    - Add jitter (±10% of interval) to distribute load
    - Handle monitor updates by rescheduling




- [x] 10. Checkpoint - Worker pool and scheduling functional 

  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Monitor service implementation
Skip test for now forcus on build feature  - [ ] 11.1 Implement monitor CRUD operations
    - CreateMonitor with validation (URL, check interval)
    - GetMonitor with caching
    - ListMonitors with filtering
    - UpdateMonitor with cache invalidation and rescheduling
    - DeleteMonitor with cleanup (cancel jobs, delete data)
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_


- [x] 12. Metrics service implementation 
  - [x] 12.1 Implement uptime percentage calculation
    - Calculate for 24h, 7d, 30d periods
    - Formula: (successful checks / total checks) × 100
    - Round to 2 decimal places
    - Handle empty history (return 0%)
    - Cache results in Redis with 1-minute TTL
    - _Requirements: 7.1, 7.3, 7.4, 7.5_
  
  - [x] 12.2 Implement response time statistics
    - Calculate average, min, max, P95, P99 for 1h, 24h, 7d periods
    - Cache results in Redis with 1-minute TTL
    - _Requirements: 8.1, 8.3_
  
  - [x] 12.3 Implement performance alerting
    - Generate alerts when response time exceeds threshold
    - _Requirements: 8.4_



- [x] 13. Data retention and aggregation
  - [x] 13.1 Implement data retention policies
    - Retain health checks and alerts for 90 days
    - Scheduled cleanup job (daily)
    - _Requirements: 8.5, 13.2, 14.2_
  
  - [x] 13.2 Implement health check aggregation
    - Aggregate checks older than 7 days into hourly summaries
    - Preserve count, success rate, avg response time
    - _Requirements: 14.4_

- [x] 14. REST API implementation
  - [x] 14.1 Implement monitor API endpoints
    - POST /api/v1/monitors (create)
    - GET /api/v1/monitors (list)
    - GET /api/v1/monitors/:id (get)
    - PUT /api/v1/monitors/:id (update)
    - DELETE /api/v1/monitors/:id (delete)
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_
  
  - [x] 14.2 Implement health check API endpoints
    - GET /api/v1/monitors/:id/checks (history with filtering and pagination)
    - GET /api/v1/monitors/:id/checks/latest (latest check)
    - _Requirements: 14.3, 14.5_
  
  - [x] 14.3 Implement alert API endpoints
    - GET /api/v1/monitors/:id/alerts (history with filtering and pagination)
    - _Requirements: 13.3, 13.4, 13.5_
  
  - [x] 14.4 Implement metrics API endpoints
    - GET /api/v1/monitors/:id/uptime (uptime percentages)
    - GET /api/v1/monitors/:id/response (response time stats)
    - _Requirements: 7.1, 8.3_
  
  - [x] 14.5 Implement system API endpoints
    - GET /health (basic health check)
    - GET /health/ready (readiness check)
    - GET /health/live (liveness check)
    - GET /metrics (Prometheus metrics)
    - _Requirements: 15.1, 15.2, 15.3_


- [x] 15. Checkpoint - REST API functional
  - Ensure all tests pass, ask the user if questions arise.

- [x] 16. WebSocket implementation
  - [x] 16.1 Implement WebSocket connection handling
    - Establish WebSocket connections on /ws
    - Maintain connection registry
    - Handle connection loss and cleanup
    - _Requirements: 9.1_
  
  - [x] 16.2 Implement real-time broadcast system
    - Broadcast health check updates to all connected clients
    - Broadcast alerts to all connected clients
    - Ensure delivery within 2 seconds
    - _Requirements: 9.2, 9.3_
  
  - [x] 16.3 Implement client-side reconnection logic
    - Automatic reconnection on connection loss
    - Exponential backoff for reconnection attempts
    - _Requirements: 9.4_



- [x] 17. Configuration management
  - [x] 17.1 Implement configuration loading
    - Load from YAML/JSON files
    - Support environment variable substitution
    - Validate configuration at startup
    - _Requirements: 11.1, 11.5_
  
  - [x] 17.2 Implement startup validation
    - Validate alert channel credentials
    - Validate database connectivity
    - Validate Redis connectivity
    - Fail fast with detailed error messages on invalid config
    - _Requirements: 11.2, 11.3, 11.4_



- [x] 18. Dashboard implementation with templ
  - [x] 18.1 Create templ templates for dashboard
    - Main dashboard layout
    - Monitor list view with status indicators
    - Monitor detail view with charts
    - Alert history view
    - _Requirements: 9.5_
  
  - [x] 18.2 Implement dashboard handlers
    - Serve dashboard HTML with templ
    - Integrate with WebSocket for real-time updates
    - Optimize for <200ms page load time
    - _Requirements: 9.5_
  
  - [x] 18.3 Add TailwindCSS styling
    - Configure TailwindCSS build process
    - Style dashboard components
    - Ensure responsive design
    - _Requirements: 9.5_


- [x] 19. Error handling and resilience
  - [x] 19.1 Implement structured logging
    - JSON structured logs with timestamp, severity, context
    - Log errors and warnings
    - _Requirements: 15.4_
  
  - [x] 19.2 Implement graceful degradation
    - Handle database connection loss (use cached data)
    - Handle Redis unavailability (fall back to database)
    - Continue operating in degraded mode on critical errors
    - _Requirements: 15.5_
  
  - [x] 19.3 Implement circuit breaker for external services
    - Open circuit after 5 consecutive failures
    - Half-open after 60 seconds
    - Close after 3 consecutive successes
    - Apply to Telegram, SMTP, Webhook adapters
    - _Requirements: 4.5, 4.6_


- [x] 20. Resource efficiency optimization
  - [x] 20.1 Optimize memory usage
    - Verify <100MB RAM with 100 monitors
    - Verify <10MB RAM when idle
    - Profile and optimize memory allocations
    - _Requirements: 12.1, 12.2_
  
  - [x] 20.2 Optimize database connection pooling
    - Configure max 20 connections
    - Tune connection pool parameters
    - _Requirements: 12.3_

- [x] 21. Docker and deployment setup
  - [x] 21.1 Create Dockerfile
    - Multi-stage build for minimal image size
    - Include templ compilation
    - Configure for production
    - _Requirements: All_
  
  - [x] 21.2 Create docker-compose.yml
    - Define app, PostgreSQL, Redis/Valkey services
    - Configure volumes for data persistence
    - Set up networking
    - _Requirements: All_
  
  - [x] 21.3 Create database migrations
    - Use golang-migrate or similar tool
    - Create migration files for all tables
    - _Requirements: 10.6, 13.1, 14.1_

- [x] 22. Documentation
  - [x] 22.1 Write setup guide
    - Installation instructions
    - Configuration guide
    - Docker deployment guide
    - _Requirements: All_
  
  - [x] 22.2 Write API documentation
    - Document all REST API endpoints
    - Include request/response examples
    - Document WebSocket message format
    - _Requirements: 10.1-10.6, 13.3, 14.3_
  
  - [x] 22.3 Write operational guide
    - Monitoring and observability
    - Troubleshooting common issues
    - Scaling recommendations
    - _Requirements: 15.1, 15.2, 15.3_

- [x] 23. Final checkpoint - Complete system integration
  - Run full test suite (unit + property tests)
  - Verify all 52 correctness properties pass
  - Test end-to-end flows with Docker Compose
  - Verify performance requirements (memory, response time, alert delivery)
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP delivery
- Each task references specific requirements for traceability
- Property tests validate universal correctness properties from the design document
- Unit tests (not explicitly listed) should focus on edge cases and integration points
- The implementation follows Clean Architecture, building from domain layer outward
- Testing is integrated throughout to catch errors early
- Checkpoints ensure incremental validation at key milestones
