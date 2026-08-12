import logging

from celery import shared_task
from django.utils import timezone

from jobs.models import Job

from .models import AlertEvent, AlertRule
from .notifiers import send_email_alert, send_sms_alert, send_slack_alert

logger = logging.getLogger(__name__)


@shared_task
def check_missed_heartbeats():
    """Runs on a beat schedule (see CELERY_BEAT_SCHEDULE). Compares every
    active job's last heartbeat against its expected interval + grace period.
    Opens a new AlertEvent (and notifies) the first time a job goes late, and
    auto-resolves it once heartbeats resume — never spams the same outage.
    """
    now = timezone.now()
    flagged = 0

    for job in Job.objects.filter(is_active=True):
        status = job.computed_status
        open_event = job.alert_events.filter(status=AlertEvent.Status.OPEN).first()

        if status in (Job.Status.LATE, Job.Status.FAILED):
            if open_event is None:
                reason = _build_reason(job, status, now)
                event = AlertEvent.objects.create(job=job, reason=reason)
                notified = _notify(job, reason)
                event.notified = notified
                event.save(update_fields=["notified"])
                flagged += 1
                logger.info("Opened alert for job=%s reason=%s", job.name, reason)
        elif open_event is not None:
            open_event.status = AlertEvent.Status.RESOLVED
            open_event.resolved_at = now
            open_event.save(update_fields=["status", "resolved_at"])
            logger.info("Resolved alert for job=%s", job.name)

    return {"jobs_flagged": flagged}


def _build_reason(job: Job, status: str, now) -> str:
    if status == Job.Status.FAILED:
        return f"Job '{job.name}' reported a failed heartbeat."
    if job.last_heartbeat_at is None:
        return f"Job '{job.name}' has never sent a heartbeat."
    elapsed = int((now - job.last_heartbeat_at).total_seconds())
    return (
        f"Job '{job.name}' missed its heartbeat: last seen {elapsed}s ago, "
        f"expected every {job.expected_interval_seconds}s "
        f"(+{job.grace_period_seconds}s grace)."
    )


def _notify(job: Job, reason: str) -> bool:
    rules = job.alert_rules.filter(is_active=True)
    if not rules.exists():
        # No explicit rule configured: fall back to Slack so silent failures
        # never go completely unnoticed.
        return send_slack_alert(f":rotating_light: {reason}")

    any_sent = False
    for rule in rules:
        if rule.channel == AlertRule.Channel.SLACK:
            sent = send_slack_alert(f":rotating_light: {reason}", webhook_url=rule.target or None)
        elif rule.channel == AlertRule.Channel.EMAIL:
            sent = send_email_alert(
                subject=f"[PipelineOps] {job.name} is unhealthy",
                message=reason,
                recipient=rule.target or None,
            )
        elif rule.channel == AlertRule.Channel.SMS:
            sent = send_sms_alert(reason, recipient=rule.target or None)
        else:
            sent = False
        any_sent = any_sent or sent
    return any_sent
