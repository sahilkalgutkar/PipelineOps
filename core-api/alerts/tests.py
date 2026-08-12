from datetime import timedelta

from django.test import TestCase
from django.utils import timezone

from alerts.models import AlertEvent
from alerts.tasks import check_missed_heartbeats
from jobs.models import Job


class CheckMissedHeartbeatsTests(TestCase):
    def setUp(self):
        self.job = Job.objects.create(
            name="nightly-etl",
            expected_interval_seconds=3600,
            grace_period_seconds=60,
            last_heartbeat_at=timezone.now() - timedelta(seconds=3700),
            last_heartbeat_status=Job.Status.HEALTHY,
        )

    def test_opens_alert_event_for_late_job(self):
        result = check_missed_heartbeats()

        self.assertEqual(result["jobs_flagged"], 1)
        event = self.job.alert_events.get()
        self.assertEqual(event.status, AlertEvent.Status.OPEN)
        self.assertIn("missed its heartbeat", event.reason)

    def test_does_not_duplicate_open_event_on_repeated_runs(self):
        check_missed_heartbeats()
        check_missed_heartbeats()

        self.assertEqual(self.job.alert_events.count(), 1)

    def test_resolves_event_once_job_is_healthy_again(self):
        check_missed_heartbeats()

        self.job.last_heartbeat_at = timezone.now()
        self.job.save(update_fields=["last_heartbeat_at"])

        check_missed_heartbeats()

        event = self.job.alert_events.get()
        self.assertEqual(event.status, AlertEvent.Status.RESOLVED)
        self.assertIsNotNone(event.resolved_at)

    def test_ignores_inactive_jobs(self):
        self.job.is_active = False
        self.job.save(update_fields=["is_active"])

        result = check_missed_heartbeats()

        self.assertEqual(result["jobs_flagged"], 0)
        self.assertEqual(self.job.alert_events.count(), 0)
