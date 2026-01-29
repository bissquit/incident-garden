# Demo Data Migration

This migration adds demonstration data to showcase IncidentGarden functionality out of the box.

## What's Included

### Service Groups (2)

1. **Core Platform** (`core-platform`)
   - Essential backend services that power the application
   - Contains: API Gateway, Authentication Service, Database Cluster

2. **User-Facing Apps** (`user-facing-apps`)
   - Customer-facing applications and interfaces
   - Contains: Web Application, Mobile API

### Services (7)

**In Core Platform group:**
- API Gateway (`api-gateway`) - Main API gateway handling all incoming requests
- Authentication Service (`auth-service`) - User authentication and authorization
- Database Cluster (`database-cluster`) - Primary PostgreSQL database cluster

**In User-Facing Apps group:**
- Web Application (`web-app`) - Main web application interface
- Mobile API (`mobile-api`) - API endpoints for mobile applications

**Standalone services:**
- CDN (`cdn`) - Content Delivery Network for static assets
- Payment Gateway (`payment-gateway`) - Third-party payment processing integration

### Event Templates (2)

1. **incident-notification** - Template for incident notifications
2. **maintenance-notification** - Template for maintenance notifications

### Events & Incidents

#### 1. Web Application Experiencing High Latency
- **Type:** Incident
- **Status:** Resolved
- **Severity:** Major
- **Affected:** Web Application (standalone service)
- **Timeline:** Started 3 hours ago, resolved 30 minutes ago
- **Updates:** 4 (investigating → identified → monitoring → resolved)
- **Demonstrates:** Complete incident lifecycle with resolution

#### 2. Core Platform Services Degraded
- **Type:** Incident
- **Status:** Resolved
- **Severity:** Critical
- **Affected:** All Core Platform services (API Gateway, Auth Service, Database)
- **Timeline:** Started 2 days ago, resolved ~2 days ago
- **Updates:** 3 (investigating → identified → resolved)
- **Demonstrates:** Group-wide incident affecting multiple services

#### 3. CDN Cache Invalidation Delays
- **Type:** Incident
- **Status:** Monitoring (active)
- **Severity:** Minor
- **Affected:** CDN
- **Timeline:** Started 45 minutes ago, still ongoing
- **Updates:** 3 (investigating → identified → monitoring)
- **Demonstrates:** Current active incident in monitoring phase

#### 4. Database Cluster Upgrade
- **Type:** Maintenance
- **Status:** Completed
- **Affected:** Database Cluster
- **Timeline:** Completed 5 days ago
- **Updates:** 3 (scheduled → in_progress → completed)
- **Demonstrates:** Completed maintenance window

#### 5. API Gateway Security Updates
- **Type:** Maintenance
- **Status:** Scheduled
- **Affected:** API Gateway
- **Timeline:** Scheduled for 2 days from now
- **Updates:** 1 (scheduled)
- **Uses:** maintenance-notification template
- **Demonstrates:** Upcoming scheduled maintenance

## Use Cases Demonstrated

✅ **Service Groups** - Organizing services into logical groups
✅ **Standalone Services** - Services not in any group
✅ **Incident Lifecycle** - Full progression from investigating to resolved
✅ **Group-wide Incidents** - Incidents affecting multiple services
✅ **Active Incidents** - Ongoing incidents in monitoring phase
✅ **Multiple Updates** - Status progression with detailed updates
✅ **Severity Levels** - Minor, major, and critical severities
✅ **Maintenance Windows** - Both completed and scheduled maintenance
✅ **Event Templates** - Template usage for notifications

## Cleanup

To remove all demo data, run:

```bash
make migrate-down
```

This will rollback to version 5 and remove all demo data while preserving the default users.

## Notes

- All events have realistic timestamps relative to the current time
- Updates show realistic incident progression
- One incident is intentionally left in "monitoring" status to show active incidents
- Future maintenance is scheduled 2 days ahead to demonstrate upcoming events
- Events are created by operator@ and admin@ demo users
