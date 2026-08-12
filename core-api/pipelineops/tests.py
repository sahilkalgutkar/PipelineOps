from django.contrib.auth.models import User
from django.test import Client, TestCase


class AuthFlowTests(TestCase):
    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")

    def test_me_unauthenticated_returns_401(self):
        response = self.client.get("/api/auth/me/")
        self.assertEqual(response.status_code, 401)

    def test_me_response_sets_csrf_cookie_even_when_unauthenticated(self):
        self.client.get("/api/auth/me/")
        self.assertIn("csrftoken", self.client.cookies)

    def test_login_with_valid_credentials_establishes_session(self):
        response = self.client.post(
            "/api/auth/login/",
            {"username": "admin", "password": "correct-horse"},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["username"], "admin")
        self.assertIn("sessionid", response.cookies)
        self.assertTrue(response.cookies["sessionid"]["httponly"])

    def test_login_with_invalid_credentials_returns_401_and_no_session(self):
        response = self.client.post(
            "/api/auth/login/",
            {"username": "admin", "password": "wrong"},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 401)
        self.assertNotIn("sessionid", response.cookies)

    def test_me_reflects_logged_in_user_after_login(self):
        self.client.post(
            "/api/auth/login/",
            {"username": "admin", "password": "correct-horse"},
            content_type="application/json",
        )
        response = self.client.get("/api/auth/me/")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["username"], "admin")

    def test_logout_clears_session(self):
        self.client.post(
            "/api/auth/login/",
            {"username": "admin", "password": "correct-horse"},
            content_type="application/json",
        )
        logout_response = self.client.post("/api/auth/logout/")
        self.assertEqual(logout_response.status_code, 204)

        me_response = self.client.get("/api/auth/me/")
        self.assertEqual(me_response.status_code, 401)


class CsrfEnforcementTests(TestCase):
    """Uses a CSRF-checking client (Django's default test Client bypasses
    CSRF entirely) to prove a logged-in session can't be used to write
    data without the matching X-CSRFToken header — the actual property
    this whole auth change exists to guarantee."""

    def setUp(self):
        self.user = User.objects.create_user(username="admin", password="correct-horse")
        self.client = Client(enforce_csrf_checks=True)

    def _login(self):
        # A same-site GET before login mirrors what the SPA does: prime the
        # csrftoken cookie via /me/, then use it on the login POST itself.
        self.client.get("/api/auth/me/")
        csrf_token = self.client.cookies["csrftoken"].value
        return self.client.post(
            "/api/auth/login/",
            {"username": "admin", "password": "correct-horse"},
            content_type="application/json",
            HTTP_X_CSRFTOKEN=csrf_token,
        )

    def test_state_changing_request_without_csrf_token_is_rejected(self):
        self._login()
        response = self.client.post(
            "/api/jobs/",
            {"name": "nightly-etl", "expected_interval_seconds": 3600},
            content_type="application/json",
        )
        self.assertEqual(response.status_code, 403)

    def test_state_changing_request_with_csrf_token_succeeds(self):
        self._login()
        csrf_token = self.client.cookies["csrftoken"].value
        response = self.client.post(
            "/api/jobs/",
            {"name": "nightly-etl", "expected_interval_seconds": 3600},
            content_type="application/json",
            HTTP_X_CSRFTOKEN=csrf_token,
        )
        self.assertEqual(response.status_code, 201)
