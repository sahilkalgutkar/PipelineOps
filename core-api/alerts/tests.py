from datetime import timedelta
from unittest.mock import MagicMock, patch

import requests
from django.contrib.auth.models import User
from django.test import TestCase, override_settings
from django.utils import timezone

from alerts.models import AlertEvent, AlertRule
from alerts.notifiers import send_email_alert, send_sms_alert, send_slack_alert
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


@override_settings(SLACK_WEBHOOK_URL="")
class SendSlackAlertTests(TestCase):
    def test_returns_false_and_skips_send_when_no_webhook_configured(self):
        with patch("alerts.notifiers.requests.post") as mock_post:
            result = send_slack_alert("job is late")

        self.assertFalse(result)
        mock_post.assert_not_called()

    @override_settings(SLACK_WEBHOOK_URL="https://hooks.slack.example/T000/B000/xxx")
    def test_posts_to_the_configured_webhook_and_returns_true_on_success(self):
        mock_response = MagicMock()
        mock_response.raise_for_status.return_value = None
        with patch("alerts.notifiers.requests.post", return_value=mock_response) as mock_post:
            result = send_slack_alert("job is late")

        self.assertTrue(result)
        mock_post.assert_called_once_with(
            "https://hooks.slack.example/T000/B000/xxx",
            json={"text": "job is late"},
            timeout=5,
        )

    def test_uses_the_explicit_webhook_url_override_when_given(self):
        mock_response = MagicMock()
        mock_response.raise_for_status.return_value = None
        with patch("alerts.notifiers.requests.post", return_value=mock_response) as mock_post:
            result = send_slack_alert("job is late", webhook_url="https://hooks.slack.example/override")

        self.assertTrue(result)
        self.assertEqual(mock_post.call_args.args[0], "https://hooks.slack.example/override")

    @override_settings(SLACK_WEBHOOK_URL="https://hooks.slack.example/T000/B000/xxx")
    def test_returns_false_when_the_request_fails(self):
        with patch(
            "alerts.notifiers.requests.post",
            side_effect=requests.RequestException("boom"),
        ):
            result = send_slack_alert("job is late")

        self.assertFalse(result)

    @override_settings(SLACK_WEBHOOK_URL="https://hooks.slack.example/T000/B000/xxx")
    def test_returns_false_when_the_webhook_responds_with_an_error_status(self):
        mock_response = MagicMock()
        mock_response.raise_for_status.side_effect = requests.HTTPError("500")
        with patch("alerts.notifiers.requests.post", return_value=mock_response):
            result = send_slack_alert("job is late")

        self.assertFalse(result)


@override_settings(ALERT_EMAIL_RECIPIENTS=[])
class SendEmailAlertTests(TestCase):
    def test_returns_false_and_skips_send_when_no_recipients_configured(self):
        with patch("alerts.notifiers.send_mail") as mock_send_mail:
            result = send_email_alert("subject", "message")

        self.assertFalse(result)
        mock_send_mail.assert_not_called()

    @override_settings(ALERT_EMAIL_RECIPIENTS=["oncall@example.com"])
    def test_sends_to_configured_recipients_and_returns_true(self):
        with patch("alerts.notifiers.send_mail") as mock_send_mail:
            result = send_email_alert("subject", "message")

        self.assertTrue(result)
        mock_send_mail.assert_called_once()
        self.assertEqual(mock_send_mail.call_args.args[0], "subject")
        self.assertEqual(mock_send_mail.call_args.args[3], ["oncall@example.com"])

    def test_uses_the_explicit_recipient_override_when_given(self):
        with patch("alerts.notifiers.send_mail") as mock_send_mail:
            result = send_email_alert("subject", "message", recipient="direct@example.com")

        self.assertTrue(result)
        self.assertEqual(mock_send_mail.call_args.args[3], ["direct@example.com"])

    @override_settings(ALERT_EMAIL_RECIPIENTS=["oncall@example.com"])
    def test_returns_false_when_sending_raises(self):
        with patch("alerts.notifiers.send_mail", side_effect=Exception("smtp down")):
            result = send_email_alert("subject", "message")

        self.assertFalse(result)


@override_settings(TWILIO_ACCOUNT_SID="", TWILIO_AUTH_TOKEN="", ALERT_SMS_RECIPIENTS=[])
class SendSmsAlertTests(TestCase):
    def test_returns_false_when_twilio_is_not_configured(self):
        result = send_sms_alert("job is late")
        self.assertFalse(result)

    @override_settings(
        TWILIO_ACCOUNT_SID="AC123",
        TWILIO_AUTH_TOKEN="secret",
        ALERT_SMS_RECIPIENTS=[],
    )
    def test_returns_false_when_twilio_configured_but_no_recipients(self):
        result = send_sms_alert("job is late")
        self.assertFalse(result)

    @override_settings(
        TWILIO_ACCOUNT_SID="AC123",
        TWILIO_AUTH_TOKEN="secret",
        TWILIO_FROM_NUMBER="+15550000000",
        ALERT_SMS_RECIPIENTS=["+15551234567"],
    )
    def test_sends_via_twilio_client_and_returns_true(self):
        mock_client_instance = MagicMock()
        with patch("twilio.rest.Client", return_value=mock_client_instance) as mock_client_cls:
            result = send_sms_alert("job is late")

        self.assertTrue(result)
        mock_client_cls.assert_called_once_with("AC123", "secret")
        mock_client_instance.messages.create.assert_called_once_with(
            body="job is late", from_="+15550000000", to="+15551234567"
        )

    @override_settings(
        TWILIO_ACCOUNT_SID="AC123",
        TWILIO_AUTH_TOKEN="secret",
        TWILIO_FROM_NUMBER="+15550000000",
        ALERT_SMS_RECIPIENTS=[],
    )
    def test_uses_the_explicit_recipient_override_when_given(self):
        mock_client_instance = MagicMock()
        with patch("twilio.rest.Client", return_value=mock_client_instance):
            result = send_sms_alert("job is late", recipient="+15559998888")

        self.assertTrue(result)
        mock_client_instance.messages.create.assert_called_once_with(
            body="job is late", from_="+15550000000", to="+15559998888"
        )

    @override_settings(
        TWILIO_ACCOUNT_SID="AC123",
        TWILIO_AUTH_TOKEN="secret",
        ALERT_SMS_RECIPIENTS=["+15551234567"],
    )
    def test_returns_false_when_twilio_raises(self):
        with patch("twilio.rest.Client", side_effect=Exception("twilio down")):
            result = send_sms_alert("job is late")

        self.assertFalse(result)


class AlertEventViewSetFilterTests(TestCase):
    """Covers AlertEventViewSet.get_queryset's job/status query-param filtering."""

    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")
        self.client.force_login(self.user)

        self.job_a = Job.objects.create(name="job-a", expected_interval_seconds=3600)
        self.job_b = Job.objects.create(name="job-b", expected_interval_seconds=3600)

        self.open_event_a = AlertEvent.objects.create(job=self.job_a, reason="late", status=AlertEvent.Status.OPEN)
        self.resolved_event_a = AlertEvent.objects.create(
            job=self.job_a, reason="late (resolved)", status=AlertEvent.Status.RESOLVED
        )
        self.open_event_b = AlertEvent.objects.create(job=self.job_b, reason="late", status=AlertEvent.Status.OPEN)

    def test_lists_all_events_with_no_filters(self):
        response = self.client.get("/api/alert-events/")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["count"], 3)

    def test_filters_by_job(self):
        response = self.client.get(f"/api/alert-events/?job={self.job_a.id}")
        self.assertEqual(response.status_code, 200)
        ids = {event["id"] for event in response.json()["results"]}
        self.assertEqual(ids, {self.open_event_a.id, self.resolved_event_a.id})

    def test_filters_by_status(self):
        response = self.client.get("/api/alert-events/?status=open")
        self.assertEqual(response.status_code, 200)
        ids = {event["id"] for event in response.json()["results"]}
        self.assertEqual(ids, {self.open_event_a.id, self.open_event_b.id})

    def test_filters_by_job_and_status_combined(self):
        response = self.client.get(f"/api/alert-events/?job={self.job_a.id}&status=resolved")
        self.assertEqual(response.status_code, 200)
        ids = {event["id"] for event in response.json()["results"]}
        self.assertEqual(ids, {self.resolved_event_a.id})


class AlertRuleViewSetTests(TestCase):
    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")
        self.client.force_login(self.user)
        self.job = Job.objects.create(name="nightly-etl", expected_interval_seconds=3600)

    def test_create_alert_rule_via_api(self):
        response = self.client.post(
            "/api/alert-rules/",
            {"job": str(self.job.id), "channel": "slack", "target": "#alerts"},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 201)
        self.assertEqual(AlertRule.objects.count(), 1)
        self.assertEqual(response.json()["job_name"], "nightly-etl")

    def test_unauthenticated_request_is_rejected(self):
        self.client.logout()
        response = self.client.get("/api/alert-rules/")
        self.assertEqual(response.status_code, 401)
