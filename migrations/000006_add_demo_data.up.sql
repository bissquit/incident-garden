-- Demo data for IncidentGarden
-- Shows: services, groups, incidents, maintenance, updates

-- ============================================================================
-- SERVICE GROUPS
-- ============================================================================

INSERT INTO service_groups (id, name, slug, description, "order")
VALUES
    ('10000000-0000-0000-0000-000000000001', 'Core Platform', 'core-platform',
     'Essential backend services that power the application', 1),
    ('10000000-0000-0000-0000-000000000002', 'User-Facing Apps', 'user-facing-apps',
     'Customer-facing applications and interfaces', 2);

-- ============================================================================
-- SERVICES
-- ============================================================================

-- Services in Core Platform group
INSERT INTO services (id, name, slug, description, status, group_id, "order")
VALUES
    ('20000000-0000-0000-0000-000000000001', 'API Gateway', 'api-gateway',
     'Main API gateway handling all incoming requests', 'operational',
     '10000000-0000-0000-0000-000000000001', 1),

    ('20000000-0000-0000-0000-000000000002', 'Authentication Service', 'auth-service',
     'User authentication and authorization', 'operational',
     '10000000-0000-0000-0000-000000000001', 2),

    ('20000000-0000-0000-0000-000000000003', 'Database Cluster', 'database-cluster',
     'Primary PostgreSQL database cluster', 'operational',
     '10000000-0000-0000-0000-000000000001', 3);

-- Services in User-Facing Apps group
INSERT INTO services (id, name, slug, description, status, group_id, "order")
VALUES
    ('20000000-0000-0000-0000-000000000004', 'Web Application', 'web-app',
     'Main web application interface', 'operational',
     '10000000-0000-0000-0000-000000000002', 1),

    ('20000000-0000-0000-0000-000000000005', 'Mobile API', 'mobile-api',
     'API endpoints for mobile applications', 'operational',
     '10000000-0000-0000-0000-000000000002', 2);

-- Standalone services
INSERT INTO services (id, name, slug, description, status, group_id, "order")
VALUES
    ('20000000-0000-0000-0000-000000000006', 'CDN', 'cdn',
     'Content Delivery Network for static assets', 'operational', NULL, 1),

    ('20000000-0000-0000-0000-000000000007', 'Payment Gateway', 'payment-gateway',
     'Third-party payment processing integration', 'operational', NULL, 2);

-- ============================================================================
-- EVENT TEMPLATES
-- ============================================================================

INSERT INTO event_templates (id, slug, type, title_template, body_template)
VALUES
    ('30000000-0000-0000-0000-000000000001', 'incident-notification', 'incident',
     '[{{.Severity}}] {{.ServiceName}} - Incident Detected',
     'We are currently investigating an issue affecting {{.ServiceName}}. Our team is working to resolve this as quickly as possible. Started at: {{.StartedAt}}'),

    ('30000000-0000-0000-0000-000000000002', 'maintenance-notification', 'maintenance',
     'Scheduled Maintenance: {{.ServiceName}}',
     'We will be performing scheduled maintenance on {{.ServiceName}}. Scheduled window: {{.ScheduledStart}} - {{.ScheduledEnd}}. We apologize for any inconvenience.');

-- ============================================================================
-- INCIDENT 1: Resolved incident for standalone service (Web Application)
-- Demonstrates full incident lifecycle with multiple updates
-- ============================================================================

INSERT INTO events (id, title, type, status, severity, description, started_at, resolved_at,
                   notify_subscribers, created_by, created_at, updated_at)
VALUES (
    '40000000-0000-0000-0000-000000000001',
    'Web Application Experiencing High Latency',
    'incident',
    'resolved',
    'major',
    'Users are experiencing significantly slower page load times across the web application.',
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '30 minutes',
    true,
    (SELECT id FROM users WHERE email = 'operator@example.com'),
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '30 minutes'
);

-- Link incident to Web Application service
INSERT INTO event_services (event_id, service_id)
VALUES ('40000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000004');

-- Incident updates showing progression
INSERT INTO event_updates (id, event_id, status, message, notify_subscribers, created_by, created_at)
VALUES
    ('50000000-0000-0000-0000-000000000001',
     '40000000-0000-0000-0000-000000000001',
     'investigating',
     'We are investigating reports of high latency in the web application. Our monitoring systems have detected increased response times.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '3 hours'),

    ('50000000-0000-0000-0000-000000000002',
     '40000000-0000-0000-0000-000000000001',
     'identified',
     'We have identified the root cause as a database connection pool exhaustion. The team is implementing a fix.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '2 hours 30 minutes'),

    ('50000000-0000-0000-0000-000000000003',
     '40000000-0000-0000-0000-000000000001',
     'monitoring',
     'The fix has been deployed and we are monitoring the system. Initial metrics show improvement in response times.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '1 hour 15 minutes'),

    ('50000000-0000-0000-0000-000000000004',
     '40000000-0000-0000-0000-000000000001',
     'resolved',
     'All systems have returned to normal operation. Response times are back to baseline levels. We will conduct a post-mortem analysis.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '30 minutes');

-- ============================================================================
-- INCIDENT 2: Resolved incident affecting service group (Core Platform)
-- Demonstrates incident affecting multiple services
-- ============================================================================

INSERT INTO events (id, title, type, status, severity, description, started_at, resolved_at,
                   notify_subscribers, created_by, created_at, updated_at)
VALUES (
    '40000000-0000-0000-0000-000000000002',
    'Core Platform Services Degraded',
    'incident',
    'resolved',
    'critical',
    'Multiple core platform services are experiencing degraded performance due to network issues.',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '1 day 22 hours',
    true,
    (SELECT id FROM users WHERE email = 'admin@example.com'),
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '1 day 22 hours'
);

-- Link incident to all Core Platform services
INSERT INTO event_services (event_id, service_id)
VALUES
    ('40000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001'),
    ('40000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002'),
    ('40000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000003');

-- Updates for group incident
INSERT INTO event_updates (id, event_id, status, message, notify_subscribers, created_by, created_at)
VALUES
    ('50000000-0000-0000-0000-000000000005',
     '40000000-0000-0000-0000-000000000002',
     'investigating',
     'We are investigating network connectivity issues affecting our core platform services. Users may experience intermittent errors.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '2 days'),

    ('50000000-0000-0000-0000-000000000006',
     '40000000-0000-0000-0000-000000000002',
     'identified',
     'Network team has identified a routing issue in our data center. We are working with the infrastructure provider to resolve.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '1 day 23 hours 30 minutes'),

    ('50000000-0000-0000-0000-000000000007',
     '40000000-0000-0000-0000-000000000002',
     'resolved',
     'Network routing has been restored. All core platform services are operating normally. No data loss occurred.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '1 day 22 hours');

-- ============================================================================
-- INCIDENT 3: Current ongoing incident (monitoring phase)
-- Shows an active incident in progress
-- ============================================================================

INSERT INTO events (id, title, type, status, severity, description, started_at, resolved_at,
                   notify_subscribers, created_by, created_at, updated_at)
VALUES (
    '40000000-0000-0000-0000-000000000003',
    'CDN Cache Invalidation Delays',
    'incident',
    'monitoring',
    'minor',
    'CDN cache invalidation is taking longer than expected. Content updates may be delayed.',
    NOW() - INTERVAL '45 minutes',
    NULL,
    true,
    (SELECT id FROM users WHERE email = 'operator@example.com'),
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '10 minutes'
);

INSERT INTO event_services (event_id, service_id)
VALUES ('40000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000006');

INSERT INTO event_updates (id, event_id, status, message, notify_subscribers, created_by, created_at)
VALUES
    ('50000000-0000-0000-0000-000000000008',
     '40000000-0000-0000-0000-000000000003',
     'investigating',
     'We are investigating reports of delayed cache invalidation in our CDN. Static content updates may take longer to propagate.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '45 minutes'),

    ('50000000-0000-0000-0000-000000000009',
     '40000000-0000-0000-0000-000000000003',
     'identified',
     'We have identified the issue as a configuration change in the CDN provider. A fix is being applied.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '25 minutes'),

    ('50000000-0000-0000-0000-000000000010',
     '40000000-0000-0000-0000-000000000003',
     'monitoring',
     'Configuration has been corrected. We are monitoring cache invalidation times to ensure they return to normal levels.',
     true,
     (SELECT id FROM users WHERE email = 'operator@example.com'),
     NOW() - INTERVAL '10 minutes');

-- ============================================================================
-- MAINTENANCE 1: Completed scheduled maintenance
-- ============================================================================

INSERT INTO events (id, title, type, status, severity, description,
                   started_at, resolved_at, scheduled_start_at, scheduled_end_at,
                   notify_subscribers, created_by, created_at, updated_at)
VALUES (
    '40000000-0000-0000-0000-000000000004',
    'Database Cluster Upgrade',
    'maintenance',
    'completed',
    NULL,
    'Upgrading database cluster to the latest version with performance improvements and security patches.',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '4 days 22 hours',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '4 days 23 hours',
    true,
    (SELECT id FROM users WHERE email = 'admin@example.com'),
    NOW() - INTERVAL '6 days',
    NOW() - INTERVAL '4 days 22 hours'
);

INSERT INTO event_services (event_id, service_id)
VALUES ('40000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000003');

INSERT INTO event_updates (id, event_id, status, message, notify_subscribers, created_by, created_at)
VALUES
    ('50000000-0000-0000-0000-000000000011',
     '40000000-0000-0000-0000-000000000004',
     'scheduled',
     'This maintenance window is scheduled for database cluster upgrade. Expected duration: 2 hours.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '6 days'),

    ('50000000-0000-0000-0000-000000000012',
     '40000000-0000-0000-0000-000000000004',
     'in_progress',
     'Maintenance has started. Database cluster is being upgraded. Services may experience brief interruptions.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '5 days'),

    ('50000000-0000-0000-0000-000000000013',
     '40000000-0000-0000-0000-000000000004',
     'completed',
     'Maintenance completed successfully. Database cluster has been upgraded and all services are operational.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '4 days 22 hours');

-- ============================================================================
-- MAINTENANCE 2: Upcoming scheduled maintenance
-- ============================================================================

INSERT INTO events (id, title, type, status, severity, description,
                   started_at, resolved_at, scheduled_start_at, scheduled_end_at,
                   notify_subscribers, template_id, created_by, created_at, updated_at)
VALUES (
    '40000000-0000-0000-0000-000000000005',
    'API Gateway Security Updates',
    'maintenance',
    'scheduled',
    NULL,
    'Applying critical security patches to the API Gateway. Minimal service disruption expected.',
    NULL,
    NULL,
    NOW() + INTERVAL '2 days',
    NOW() + INTERVAL '2 days 1 hour',
    true,
    '30000000-0000-0000-0000-000000000002',
    (SELECT id FROM users WHERE email = 'admin@example.com'),
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
);

INSERT INTO event_services (event_id, service_id)
VALUES ('40000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000001');

INSERT INTO event_updates (id, event_id, status, message, notify_subscribers, created_by, created_at)
VALUES
    ('50000000-0000-0000-0000-000000000014',
     '40000000-0000-0000-0000-000000000005',
     'scheduled',
     'Scheduled maintenance for security updates. We will apply critical patches to ensure system security. Expected minimal impact.',
     true,
     (SELECT id FROM users WHERE email = 'admin@example.com'),
     NOW() - INTERVAL '1 day');
