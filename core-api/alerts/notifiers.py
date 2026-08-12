"""Notification channels used by the alerting worker. Each function is
best-effort: it logs and returns False on failure rather than raising, so one
broken channel doesn't stop other rules for the same job from firing.
"""

import logging

import requests
from django.conf import settings
from django.core.mail import send_mail

logger = logging.getLogger(__name__)


def send_slack_alert(message: str, webhook_url: str | None = None) -> bool:
    url = webhook_url or settings.SLACK_WEBHOOK_URL
    if not url:
        logger.warning("Slack alert skipped, no webhook configured: %s", message)
        return False
    try:
        resp = requests.post(url, json={"text": message}, timeout=5)
        resp.raise_for_status()
        return True
    except requests.RequestException:
        logger.exception("Failed to send Slack alert")
        return False


def send_email_alert(subject: str, message: str, recipient: str | None = None) -> bool:
    recipients = [recipient] if recipient else settings.ALERT_EMAIL_RECIPIENTS
    if not recipients:
        logger.warning("Email alert skipped, no recipients configured: %s", subject)
        return False
    try:
        send_mail(subject, message, settings.DEFAULT_FROM_EMAIL, recipients, fail_silently=False)
        return True
    except Exception:
        logger.exception("Failed to send email alert")
        return False


def send_sms_alert(message: str, recipient: str | None = None) -> bool:
    if not (settings.TWILIO_ACCOUNT_SID and settings.TWILIO_AUTH_TOKEN):
        logger.warning("SMS alert skipped, Twilio not configured: %s", message)
        return False
    recipients = [recipient] if recipient else settings.ALERT_SMS_RECIPIENTS
    if not recipients:
        logger.warning("SMS alert skipped, no recipients configured: %s", message)
        return False
    try:
        from twilio.rest import Client

        client = Client(settings.TWILIO_ACCOUNT_SID, settings.TWILIO_AUTH_TOKEN)
        for to in recipients:
            client.messages.create(body=message, from_=settings.TWILIO_FROM_NUMBER, to=to)
        return True
    except Exception:
        logger.exception("Failed to send SMS alert")
        return False
