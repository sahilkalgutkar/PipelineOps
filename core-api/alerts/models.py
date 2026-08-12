from django.db import models

from jobs.models import Job


class AlertRule(models.Model):
    """Where to send an alert when a job misses its heartbeat."""

    class Channel(models.TextChoices):
        SLACK = "slack", "Slack"
        EMAIL = "email", "Email"
        SMS = "sms", "SMS"

    id = models.BigAutoField(primary_key=True)
    job = models.ForeignKey(Job, on_delete=models.CASCADE, related_name="alert_rules")
    channel = models.CharField(max_length=16, choices=Channel.choices)
    target = models.CharField(
        max_length=255,
        blank=True,
        default="",
        help_text="Override destination (Slack webhook URL, email address, phone number). "
        "Blank uses the channel's default from settings.",
    )
    is_active = models.BooleanField(default=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        ordering = ["job__name", "channel"]

    def __str__(self) -> str:
        return f"{self.job.name} -> {self.channel}"


class AlertEvent(models.Model):
    """A single fired alert, so we don't spam the same missed heartbeat repeatedly."""

    class Status(models.TextChoices):
        OPEN = "open", "Open"
        RESOLVED = "resolved", "Resolved"

    id = models.BigAutoField(primary_key=True)
    job = models.ForeignKey(Job, on_delete=models.CASCADE, related_name="alert_events")
    rule = models.ForeignKey(
        AlertRule, on_delete=models.SET_NULL, null=True, blank=True, related_name="events"
    )
    reason = models.TextField()
    status = models.CharField(max_length=16, choices=Status.choices, default=Status.OPEN)
    notified = models.BooleanField(default=False)
    triggered_at = models.DateTimeField(auto_now_add=True)
    resolved_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        ordering = ["-triggered_at"]

    def __str__(self) -> str:
        return f"{self.job.name}: {self.reason[:40]}"
