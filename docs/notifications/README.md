# Notification research

[Documentation index](../README.md)

These pages preserve notification-system design research. Competitor behavior
and queue recommendations are historical inputs, not current Helpin contracts.
The [notification PRD](../prds/notifications-system.md) records requirements.

For the current implementation, start with the
[notification service](../../server/internal/service/notification.go),
[delivery repository](../../server/internal/repository/notification_email.go),
and [API startup](../../server/cmd/api/main.go). River was not adopted in the
current backend; notification digests are processed by the API's periodic sweep.

- [Linear Notification System — Research](linear-notification-system.md)
- [Notification System: Background Job Queue Comparison](notification-queue-comparison.md)
- [Shortcut (formerly Clubhouse) Notification System Research](research-shortcut-notifications.md)
