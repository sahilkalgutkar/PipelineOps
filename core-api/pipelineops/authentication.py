from rest_framework.authentication import SessionAuthentication as _SessionAuthentication


class SessionAuthentication(_SessionAuthentication):
    """DRF's SessionAuthentication.authenticate_header() returns None, which
    makes an unauthenticated request come back as 403 rather than 401 (see
    rest_framework.views.exception_handler) — indistinguishable, from the
    frontend's perspective, from a CSRF failure or an authenticated user
    hitting a resource they're not allowed to touch. Declaring a scheme here
    keeps "not logged in" a clean, unambiguous 401.
    """

    def authenticate_header(self, request):
        return "Session"
