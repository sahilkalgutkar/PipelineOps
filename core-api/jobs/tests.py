from datetime import timedelta

from django.test import TestCase
from django.utils import timezone

from jobs.models import Job


class ComputedStatusTests(TestCase):
    def setUp(self):
        self.job = Job.objects.create(
            name="nightly-etl",
            expected_interval_seconds=3600,
            grace_period_seconds=60,
        )

    def test_unknown_before_first_deadline(self):
        self.assertEqual(self.job.computed_status, Job.Status.UNKNOWN)

    def test_late_once_deadline_passes_with_no_heartbeat(self):
        self.job.created_at = timezone.now() - timedelta(seconds=3700)
        self.job.save(update_fields=["created_at"])
        self.assertEqual(self.job.computed_status, Job.Status.LATE)

    def test_healthy_within_interval_after_heartbeat(self):
        self.job.last_heartbeat_at = timezone.now()
        self.job.last_heartbeat_status = Job.Status.HEALTHY
        self.job.save(update_fields=["last_heartbeat_at", "last_heartbeat_status"])
        self.assertEqual(self.job.computed_status, Job.Status.HEALTHY)

    def test_late_once_heartbeat_ages_past_interval_and_grace(self):
        self.job.last_heartbeat_at = timezone.now() - timedelta(seconds=3700)
        self.job.last_heartbeat_status = Job.Status.HEALTHY
        self.job.save(update_fields=["last_heartbeat_at", "last_heartbeat_status"])
        self.assertEqual(self.job.computed_status, Job.Status.LATE)

    def test_failed_status_wins_regardless_of_timing(self):
        self.job.last_heartbeat_at = timezone.now()
        self.job.last_heartbeat_status = Job.Status.FAILED
        self.job.save(update_fields=["last_heartbeat_at", "last_heartbeat_status"])
        self.assertEqual(self.job.computed_status, Job.Status.FAILED)
