from datetime import timedelta

from django.contrib.auth.models import User
from django.test import TestCase
from django.utils import timezone

from jobs.models import Heartbeat, Job


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


class JobViewSetTests(TestCase):
    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")
        self.client.force_login(self.user)

    def test_creating_a_job_assigns_the_logged_in_user_as_owner(self):
        response = self.client.post(
            "/api/jobs/",
            {"name": "nightly-etl", "expected_interval_seconds": 3600},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 201)
        job = Job.objects.get(name="nightly-etl")
        self.assertEqual(job.owner, self.user)
        self.assertEqual(response.json()["owner_username"], "admin")

    def test_retrieve_uses_the_detail_serializer_with_recent_heartbeats(self):
        job = Job.objects.create(name="nightly-etl", expected_interval_seconds=3600, owner=self.user)
        Heartbeat.objects.create(job=job, status=Heartbeat.Status.SUCCESS, duration_ms=120)

        response = self.client.get(f"/api/jobs/{job.id}/")

        self.assertEqual(response.status_code, 200)
        self.assertIn("recent_heartbeats", response.json())
        self.assertEqual(len(response.json()["recent_heartbeats"]), 1)

    def test_list_does_not_include_recent_heartbeats_field(self):
        Job.objects.create(name="nightly-etl", expected_interval_seconds=3600, owner=self.user)
        response = self.client.get("/api/jobs/")
        self.assertEqual(response.status_code, 200)
        self.assertNotIn("recent_heartbeats", response.json()["results"][0])


class HeartbeatViewSetTests(TestCase):
    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")
        self.client.force_login(self.user)
        self.job_a = Job.objects.create(name="job-a", expected_interval_seconds=3600)
        self.job_b = Job.objects.create(name="job-b", expected_interval_seconds=3600)
        self.hb_a = Heartbeat.objects.create(job=self.job_a, status=Heartbeat.Status.SUCCESS)
        self.hb_b = Heartbeat.objects.create(job=self.job_b, status=Heartbeat.Status.FAILURE)

    def test_lists_all_heartbeats_with_no_filter(self):
        response = self.client.get("/api/heartbeats/")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["count"], 2)

    def test_filters_heartbeats_by_job(self):
        response = self.client.get(f"/api/heartbeats/?job={self.job_a.id}")
        self.assertEqual(response.status_code, 200)
        results = response.json()["results"]
        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["id"], self.hb_a.id)

    def test_heartbeats_are_read_only(self):
        response = self.client.post(
            "/api/heartbeats/",
            {"job": str(self.job_a.id), "status": "success"},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 405)
