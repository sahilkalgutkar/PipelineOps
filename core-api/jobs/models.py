import uuid

from django.conf import settings
from django.db import models


class Job(models.Model):
    """A scheduled job PipelineOps is watching (cron job, Airflow DAG, batch script, ...)."""

    class Status(models.TextChoices):
        UNKNOWN = "unknown", "Unknown"
        HEALTHY = "healthy", "Healthy"
        LATE = "late", "Late"
        FAILED = "failed", "Failed"

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    name = models.CharField(max_length=255, unique=True)
    description = models.TextField(blank=True, default="")
    schedule_description = models.CharField(
        max_length=255,
        blank=True,
        default="",
        help_text="Human-readable schedule, e.g. cron expression '*/15 * * * *'",
    )
    expected_interval_seconds = models.PositiveIntegerField(
        help_text="How often this job should send a heartbeat, in seconds."
    )
    grace_period_seconds = models.PositiveIntegerField(
        default=60, help_text="Extra time allowed past the expected interval before alerting."
    )
    owner = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.SET_NULL, null=True, related_name="jobs"
    )
    is_active = models.BooleanField(default=True)

    # Denormalized fields, updated directly by the Go ingestion service on
    # every heartbeat write so the dashboard can render job health without
    # scanning the full heartbeat history table.
    last_heartbeat_at = models.DateTimeField(null=True, blank=True)
    last_heartbeat_status = models.CharField(
        max_length=16, choices=Status.choices, default=Status.UNKNOWN
    )

    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        ordering = ["name"]

    def __str__(self) -> str:
        return self.name

    @property
    def computed_status(self) -> str:
        """Derive current health from the last heartbeat vs. the expected interval.

        A job that has never sent a heartbeat is only "unknown" for its first
        deadline window after creation — past that, silence is exactly the
        failure mode PipelineOps exists to catch, so it must become "late"
        rather than staying unknown forever.
        """
        from django.utils import timezone

        deadline_seconds = self.expected_interval_seconds + self.grace_period_seconds
        baseline = self.last_heartbeat_at or self.created_at
        elapsed = (timezone.now() - baseline).total_seconds()

        if self.last_heartbeat_at and self.last_heartbeat_status == self.Status.FAILED:
            return self.Status.FAILED
        if elapsed > deadline_seconds:
            return self.Status.LATE
        if not self.last_heartbeat_at:
            return self.Status.UNKNOWN
        return self.Status.HEALTHY


class Heartbeat(models.Model):
    """A single ping received from a job run. Written by the Go ingestion service."""

    class Status(models.TextChoices):
        SUCCESS = "success", "Success"
        FAILURE = "failure", "Failure"

    id = models.BigAutoField(primary_key=True)
    job = models.ForeignKey(Job, on_delete=models.CASCADE, related_name="heartbeats")
    status = models.CharField(max_length=16, choices=Status.choices, default=Status.SUCCESS)
    duration_ms = models.PositiveIntegerField(null=True, blank=True)
    metadata = models.JSONField(default=dict, blank=True)
    received_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        ordering = ["-received_at"]
        indexes = [
            models.Index(fields=["job", "-received_at"]),
        ]

    def __str__(self) -> str:
        return f"{self.job.name} @ {self.received_at.isoformat()} ({self.status})"
